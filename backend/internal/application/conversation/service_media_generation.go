package conversation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MediaImageTaskType 表示媒体图片任务类型。
type MediaImageTaskType string

const (
	// MediaImageTaskGeneration 表示纯文本提示词生成图片任务。
	MediaImageTaskGeneration MediaImageTaskType = "image_generation"
	// MediaImageTaskEdit 表示基于输入图片的编辑任务。
	MediaImageTaskEdit MediaImageTaskType = "image_edit"
	// MediaVideoTaskGeneration 表示纯文本或参考图视频生成任务。
	MediaVideoTaskGeneration MediaImageTaskType = "video_generation"
)

const maxMediaImageEditInputImages = 16

type mediaImageCapabilities struct {
	Image struct {
		Stream *bool `json:"stream"`
	} `json:"image"`
}

// MediaImageInput 定义媒体图片任务的应用层入参。
type MediaImageInput struct {
	UserID                uint
	ConversationID        uint
	RequestID             string
	TaskType              MediaImageTaskType
	Prompt                string
	PlatformModelName     string
	Options               map[string]interface{}
	ClientRunID           string
	FileIDs               []string
	MaskFileID            string
	ParentMessagePublicID string
	SourceMessagePublicID string
	BranchReason          string
	OnEvent               func(eventType string, payload map[string]interface{}) error
}

// StreamMediaImage 执行图片生成任务并把结果保存为文件对象。
// 图片能力不复用聊天生成链路，只通过图片任务类型和图片协议路由。
func (s *Service) StreamMediaImage(ctx context.Context, input MediaImageInput) (*SendMessageResult, error) {
	if input.TaskType != MediaImageTaskGeneration && input.TaskType != MediaImageTaskEdit && input.TaskType != MediaVideoTaskGeneration {
		return nil, ErrInvalidMediaGenerationTask
	}
	if s.routeResolver == nil || s.llmClient == nil {
		return nil, ErrModelRouteNotConfigured
	}
	ctx = context.WithoutCancel(ctx)

	// clientRunID 是媒体任务的幂等键；重复提交不能继续创建 run 和消息。
	runID := normalizeRunID(input.ClientRunID)
	if runID == "" {
		runID = "run_" + normalizePublicID(uuid.NewString())
	}
	existingRuns, err := s.repo.ListConversationRunsByRunIDs(ctx, input.UserID, input.ConversationID, []string{runID})
	if err != nil {
		return nil, err
	}
	if len(existingRuns) > 0 {
		return nil, ErrDuplicateMessageGenerationRun
	}
	startedAt := time.Now()
	conversation, err := s.repo.GetConversationByUser(ctx, input.ConversationID, input.UserID)
	if err != nil {
		return nil, ErrConversationNotFound
	}

	normalizedBranchReason := normalizeBranchReason(input.BranchReason)
	branchState, err := s.resolveMessageBranch(ctx, input.ConversationID, input.UserID, input.ParentMessagePublicID, input.SourceMessagePublicID, normalizedBranchReason)
	if err != nil {
		return nil, err
	}
	reuseUserMessage := branchState.ReuseUserMessage != nil
	if reuseUserMessage {
		input.Prompt = branchState.ReuseUserMessage.Content
		input.FileIDs = parseAttachmentSnapshotFileIDs(branchState.ReuseUserMessage.Attachments)
	}
	if strings.TrimSpace(input.Prompt) == "" {
		if input.TaskType == MediaVideoTaskGeneration {
			return nil, ErrMediaVideoPromptRequired
		}
		return nil, ErrMediaImagePromptRequired
	}
	if err := s.ValidatePromptSensitiveWords(input.Prompt); err != nil {
		return nil, err
	}
	if input.TaskType == MediaImageTaskGeneration && len(input.FileIDs) > 0 {
		return nil, ErrMediaImageGenerationRejectsInputs
	}
	if input.TaskType == MediaImageTaskEdit && len(input.FileIDs) == 0 {
		return nil, ErrMediaImageEditInputRequired
	}
	if input.TaskType == MediaVideoTaskGeneration && len(input.FileIDs) > 1 {
		return nil, ErrMediaVideoTooManyReferenceImages
	}

	platformModelName := strings.TrimSpace(input.PlatformModelName)
	if platformModelName == "" {
		platformModelName = strings.TrimSpace(conversation.Model)
	}
	if platformModelName == "" {
		return nil, ErrModelRouteNotConfigured
	}
	taskRouteType := channel.TaskTypeImageGeneration
	endpoint := llm.EndpointImageGenerations
	if input.TaskType == MediaImageTaskEdit {
		taskRouteType = channel.TaskTypeImageEdit
		endpoint = llm.EndpointImageEdits
	} else if input.TaskType == MediaVideoTaskGeneration {
		taskRouteType = channel.TaskTypeVideoGeneration
		endpoint = llm.EndpointVideoGenerations
	}
	route, err := s.routeResolver.ResolveRoute(ctx, channel.ResolveRouteInput{
		PlatformModelName: platformModelName,
		TaskType:          taskRouteType,
		Scope:             channel.RouteScopeUser,
		UserID:            input.UserID,
		ConversationID:    input.ConversationID,
		RequestID:         strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return nil, ErrModelRouteNotConfigured
	}
	if input.TaskType == MediaImageTaskGeneration && !llm.IsImageGenerationAdapter(route.Protocol) {
		return nil, ErrMediaRouteProtocolMismatch
	}
	if input.TaskType == MediaImageTaskEdit && !llm.IsImageEditAdapter(route.Protocol) {
		return nil, ErrMediaRouteProtocolMismatch
	}
	if input.TaskType == MediaVideoTaskGeneration && !llm.IsVideoGenerationAdapter(route.Protocol) {
		return nil, ErrMediaRouteProtocolMismatch
	}
	// 图片任务会把会话当前模型更新为实际执行的图片模型；标题、标签等内部文本任务会单独回退到聊天模型。
	if strings.TrimSpace(conversation.Model) != strings.TrimSpace(route.PlatformModelName) {
		conversation.Model = strings.TrimSpace(route.PlatformModelName)
		conversation.Provider = inferProvider(conversation.Model)
		if err = s.repo.UpdateConversationModel(ctx, input.ConversationID, conversation.Model, conversation.Provider); err != nil {
			return nil, err
		}
	}

	cfg := s.cfg.Snapshot()
	filteredOptions := filterModelOptions(input.Options, route.Protocol, modelOptionPolicyConfig{
		Mode:                  cfg.ModelOptionPolicyMode,
		AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
		DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
		ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
	})
	if input.TaskType == MediaVideoTaskGeneration {
		filteredOptions = sanitizeOpenAIVideoGenerationOptions(route.UpstreamModel, filteredOptions)
	}

	resolvedAttachments, imageEditParts, err := s.resolveMediaImageEditInputs(ctx, input)
	if err != nil {
		return nil, err
	}
	var videoReferenceParts []llm.ContentPart
	if input.TaskType == MediaVideoTaskGeneration {
		resolvedAttachments, videoReferenceParts, err = s.resolveMediaVideoReferenceInputs(ctx, input, route.UpstreamModel, filteredOptions)
		if err != nil {
			return nil, err
		}
	}
	var maskPart *llm.ContentPart
	if input.TaskType == MediaImageTaskEdit {
		maskPart, err = s.resolveMediaImageEditMask(ctx, input.UserID, input.MaskFileID)
		if err != nil {
			return nil, err
		}
	}
	attachmentsJSON := marshalAttachmentSnapshots(resolvedAttachments)

	run := &model.Run{
		RunID:              runID,
		RequestID:          strings.TrimSpace(input.RequestID),
		UserID:             input.UserID,
		ConversationID:     input.ConversationID,
		TaskType:           string(input.TaskType),
		Endpoint:           endpoint,
		Provider:           strings.TrimSpace(conversation.Provider),
		ProviderProtocol:   route.Protocol,
		UpstreamID:         route.UpstreamID,
		UpstreamModelID:    route.UpstreamModelID,
		UpstreamName:       route.UpstreamName,
		RequestedModelName: platformModelName,
		PlatformModelName:  route.PlatformModelName,
		RoutedBindingCode:  route.BindingCode,
		ModelVendor:        route.ModelVendor,
		ModelIcon:          route.ModelIcon,
		UpstreamModelName:  route.UpstreamModel,
		Status:             "error",
		StartedAt:          startedAt,
	}
	var retErr error
	defer func() {
		endedAt := time.Now()
		run.EndedAt = &endedAt
		run.TotalLatencyMS = endedAt.Sub(startedAt).Milliseconds()
		if retErr == nil {
			run.Status = "success"
		} else if errors.Is(retErr, ErrMessageGenerationCanceled) {
			run.Status = "canceled"
			run.ErrorCode = classifyRunErrorCode(retErr)
			run.ErrorMessage = truncateError(messageErrorSummary(retErr), 255)
		} else {
			run.Status = "error"
			run.ErrorCode = classifyRunErrorCode(retErr)
			run.ErrorMessage = truncateError(messageErrorSummary(retErr), 255)
		}
		if err := s.repo.CreateConversationRun(context.WithoutCancel(ctx), run); err != nil && s.logger != nil {
			s.logger.Error("create_media_conversation_run_failed",
				zap.String("trace_id", traceid.FromContext(ctx)),
				zap.String("run_id", run.RunID),
				zap.Error(err),
			)
		}
	}()
	cancelCtx, cancel := context.WithCancel(ctx)
	ctx = cancelCtx
	s.generationStreams.register(ctx, runID, input.UserID, cancel)

	assistantMessage := &model.Message{
		ConversationID: input.ConversationID,
		UserID:         input.UserID,
		PublicID:       normalizePublicID(uuid.NewString()),
		RunID:          runID,
		Role:           "assistant",
		ContentType:    mediaAssistantContentType(input.TaskType),
		Content:        "",
		BranchReason:   normalizedBranchReason,
		Status:         "pending",
		Attachments:    "[]",
	}
	var userMessage *model.Message
	if reuseUserMessage {
		reused := *branchState.ReuseUserMessage
		userMessage = &reused
		assistantMessage.ParentMessageID = &userMessage.ID
		assistantMessage.SourceMessageID = branchState.SourceMessageID
		if err = s.repo.CreateAssistantBranchMessage(ctx, assistantMessage); err != nil {
			retErr = err
			return nil, err
		}
		assistantMessage.ParentPublicID = userMessage.PublicID
		assistantMessage.SourcePublicID = branchState.SourcePublicID
	} else {
		userMessage = &model.Message{
			ConversationID:  input.ConversationID,
			UserID:          input.UserID,
			PublicID:        normalizePublicID(uuid.NewString()),
			ParentMessageID: branchState.ParentMessageID,
			RunID:           runID,
			Role:            "user",
			ContentType:     mediaUserContentType(input.TaskType, len(resolvedAttachments) > 0),
			Content:         strings.TrimSpace(input.Prompt),
			BranchReason:    normalizedBranchReason,
			SourceMessageID: branchState.SourceMessageID,
			TokenUsage:      estimateTokens(input.Prompt),
			InputTokens:     estimateTokens(input.Prompt),
			Status:          "success",
			Attachments:     attachmentsJSON,
		}
		userAttachmentRows := make([]model.Attachment, 0, len(resolvedAttachments))
		if len(resolvedAttachments) > 0 {
			now := time.Now()
			for _, item := range resolvedAttachments {
				userAttachmentRows = append(userAttachmentRows, model.Attachment{
					ConversationID: input.ConversationID,
					UserID:         input.UserID,
					FileID:         strings.TrimSpace(item.FileID),
					Kind:           normalizeAttachmentKind(item.Kind, item.MimeType),
					FileName:       strings.TrimSpace(item.FileName),
					MimeType:       strings.TrimSpace(item.MimeType),
					FileSize:       item.FileSize,
					SHA256:         strings.TrimSpace(item.SHA256),
					StoragePath:    strings.TrimSpace(item.StoragePath),
					Status:         "active",
					MetaJSON:       strings.TrimSpace(item.MetaJSON),
					UploadedAt:     now,
				})
			}
		}

		// 媒体任务同样产生一个完整消息回合，初始本地写入必须原子提交。
		if err = s.repo.CreateMessagePairWithUserAttachments(ctx, userMessage, assistantMessage, userAttachmentRows); err != nil {
			retErr = err
			return nil, err
		}
		userMessage.ParentPublicID = branchState.ParentPublicID
		userMessage.SourcePublicID = branchState.SourcePublicID
		assistantMessage.ParentPublicID = userMessage.PublicID
		s.maybeGenerateConversationMetadataAsync(*conversation, *userMessage)
	}
	traceRecorder := newMessageTraceRecorder(s, ctx, assistantMessage, input.OnEvent)
	defer func() {
		if retErr != nil && traceRecorder != nil {
			traceRecorder.fail(retErr)
			traceRecorder.attachToMessage(assistantMessage)
		}
	}()
	emitMediaEvent(input.OnEvent, "queued", mediaQueuedMessage(input.TaskType))

	attributionReferer, attributionTitle := s.llmAttribution()
	routeConfig := llm.RouteConfig{
		Protocol:            route.Protocol,
		BaseURL:             route.BaseURL,
		APIKey:              route.APIKey,
		HeadersJSON:         route.HeadersJSON,
		ConnectTimeoutMS:    route.ConnectTimeoutMS,
		ReadTimeoutMS:       route.ReadTimeoutMS,
		StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
		Endpoint:            endpoint,
		UpstreamModel:       route.UpstreamModel,
		AttributionReferer:  attributionReferer,
		AttributionTitle:    attributionTitle,
	}
	filteredOptions = filterModelOptions(input.Options, route.Protocol, modelOptionPolicyConfig{
		Mode:                  cfg.ModelOptionPolicyMode,
		AllowedPathsJSON:      cfg.ModelOptionAllowedPaths,
		DeniedPathsJSON:       cfg.ModelOptionDeniedPaths,
		ModelCapabilitiesJSON: route.ModelCapabilitiesJSON,
	})
	if llm.NormalizeAdapter(route.Protocol) == llm.AdapterGeminiInteractions {
		filteredOptions = withGeminiInteractionResponseType(filteredOptions, "image")
		routeConfig.Endpoint = llm.EndpointInteractions
		run.Endpoint = llm.EndpointInteractions
	}

	emitMediaEvent(input.OnEvent, "running", mediaImageRunningMessage(input.TaskType))
	generateInput := llm.GenerateInput{
		RequestID:      strings.TrimSpace(input.RequestID),
		ConversationID: input.ConversationID,
		Messages: []llm.Message{{
			Role:    "user",
			Content: strings.TrimSpace(input.Prompt),
		}},
		Options: filteredOptions,
	}
	if input.TaskType == MediaImageTaskEdit {
		parts := make([]llm.ContentPart, 0, 1+len(imageEditParts))
		parts = append(parts, llm.ContentPart{
			Kind: llm.ContentPartText,
			Text: strings.TrimSpace(input.Prompt),
		})
		parts = append(parts, imageEditParts...)
		generateInput.Messages = []llm.Message{{
			Role:  "user",
			Parts: parts,
		}}
		generateInput.ImageEditMask = maskPart
	}
	if input.TaskType == MediaVideoTaskGeneration && len(videoReferenceParts) > 0 {
		parts := make([]llm.ContentPart, 0, 1+len(videoReferenceParts))
		parts = append(parts, llm.ContentPart{
			Kind: llm.ContentPartText,
			Text: strings.TrimSpace(input.Prompt),
		})
		parts = append(parts, videoReferenceParts...)
		generateInput.Messages = []llm.Message{{
			Role:  "user",
			Parts: parts,
		}}
	}
	var output *llm.GenerateOutput
	useMediaImageStream := mediaImageStreamEnabled(routeConfig.Protocol, routeConfig.UpstreamModel, route.ModelCapabilitiesJSON) &&
		shouldUseUpstreamMediaImageStream(routeConfig.Protocol, routeConfig.UpstreamModel, filteredOptions)
	if s.isMessageGenerationCanceled(context.Background(), runID) {
		retErr = ErrMessageGenerationCanceled
		return nil, retErr
	}
	if input.TaskType == MediaVideoTaskGeneration || useMediaImageStream {
		output, err = s.llmClient.GenerateStream(ctx, routeConfig, generateInput, func(event llm.GenerateStreamEvent) error {
			if event.Usage != (llm.Usage{}) && input.OnEvent != nil {
				if streamErr := input.OnEvent("usage", map[string]interface{}{
					"input_tokens":       event.Usage.InputTokens,
					"output_tokens":      event.Usage.OutputTokens,
					"cache_read_tokens":  event.Usage.CacheReadTokens,
					"cache_write_tokens": event.Usage.CacheWriteTokens,
					"reasoning_tokens":   event.Usage.ReasoningTokens,
				}); streamErr != nil {
					return streamErr
				}
			}
			if event.GeneratedImage != nil && event.GeneratedImagePartial {
				return emitMediaImageDelta(input.OnEvent, event)
			}
			if event.GeneratedVideoStatus != nil {
				return emitMediaVideoStatus(input.OnEvent, event.GeneratedVideoStatus)
			}
			return nil
		})
	} else {
		output, err = s.llmClient.Generate(ctx, routeConfig, generateInput)
	}
	if err != nil {
		if s.isCanceledMediaGeneration(ctx, runID, err) {
			retErr = ErrMessageGenerationCanceled
			result, cancelErr := s.completeCanceledMediaGeneration(canceledMediaGenerationInput{
				Context:          ctx,
				Conversation:     conversation,
				UserMessage:      userMessage,
				AssistantMessage: assistantMessage,
				ReuseUserMessage: reuseUserMessage,
				Route:            *route,
				EffectiveOptions: filteredOptions,
				GenerateInput:    generateInput,
				StartedAt:        startedAt,
			})
			if cancelErr != nil {
				retErr = cancelErr
				return nil, cancelErr
			}
			applyCanceledMediaRunUsage(run, result)
			return result, nil
		}
		s.routeResolver.MarkRouteFailure(ctx, route, err)
		retErr = wrapUpstreamRequestError(err)
		_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
		return nil, retErr
	}
	s.routeResolver.MarkRouteSuccess(ctx, route)
	if input.TaskType == MediaVideoTaskGeneration {
		if output == nil || len(output.GeneratedVideos) == 0 {
			retErr = ErrUpstreamEmptyResponse
			_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
			return nil, retErr
		}
		result, saveErr := s.completeMediaVideoGeneration(ctx, mediaVideoCompletionInput{
			Input:            input,
			Conversation:     conversation,
			UserMessage:      userMessage,
			AssistantMessage: assistantMessage,
			Route:            route,
			FilteredOptions:  filteredOptions,
			Output:           output,
			StartedAt:        startedAt,
		})
		if saveErr != nil {
			retErr = saveErr
			return nil, saveErr
		}
		usage := output.Usage
		run.InputTokens = usage.InputTokens
		run.OutputTokens = usage.OutputTokens
		run.CacheReadTokens = usage.CacheReadTokens
		run.CacheWriteTokens = usage.CacheWriteTokens
		run.ReasoningTokens = usage.ReasoningTokens
		return result, nil
	}
	if output == nil || len(output.GeneratedImages) == 0 {
		retErr = ErrUpstreamEmptyResponse
		_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
		return nil, retErr
	}

	emitMediaEvent(input.OnEvent, "saving_artifact", "saving image")
	uploaded := make([]model.FileObject, 0, len(output.GeneratedImages))
	attachmentRows := make([]model.Attachment, 0, len(output.GeneratedImages))
	now := time.Now()
	for i, image := range output.GeneratedImages {
		data, mimeType, readErr := s.readGeneratedImage(ctx, image)
		if readErr != nil {
			retErr = readErr
			_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
			return nil, readErr
		}
		fileName := generatedImageFileName(route.PlatformModelName, now, i, len(output.GeneratedImages), mimeType)
		uploadResult, uploadErr := s.UploadFile(ctx, appupload.UploadFileInput{
			UserID:       input.UserID,
			Purpose:      "generated_image",
			FileName:     fileName,
			MimeType:     mimeType,
			DeclaredSize: int64(len(data)),
			Reader:       bytes.NewReader(data),
		})
		if uploadErr != nil {
			retErr = uploadErr
			_ = s.repo.UpdateMessageState(ctx, assistantMessage.ID, "error", classifyRunErrorCode(retErr), truncateError(messageErrorSummary(retErr), 255))
			return nil, uploadErr
		}
		file := uploadResult.File
		uploaded = append(uploaded, file)
		attachmentRows = append(attachmentRows, model.Attachment{
			ConversationID: input.ConversationID,
			MessageID:      assistantMessage.ID,
			UserID:         input.UserID,
			FileID:         file.FileID,
			Kind:           "image",
			FileName:       file.FileName,
			MimeType:       file.DetectedMIME,
			FileSize:       file.SizeBytes,
			SHA256:         file.SHA256,
			StoragePath:    file.StoragePath,
			Status:         "active",
			UploadedAt:     now,
		})
	}
	usage := output.Usage
	if reuseUserMessage {
		assistantMessage.InputTokens = usage.InputTokens
		assistantMessage.CacheReadTokens = usage.CacheReadTokens
		assistantMessage.CacheWriteTokens = usage.CacheWriteTokens
	} else {
		userMessage.InputTokens = usage.InputTokens
		userMessage.CacheReadTokens = usage.CacheReadTokens
		userMessage.CacheWriteTokens = usage.CacheWriteTokens
		userMessage.TokenUsage = usage.InputTokens + usage.CacheReadTokens + usage.CacheWriteTokens
	}

	content := generatedImageMarkdown(uploaded)
	latencyMS := time.Since(startedAt).Milliseconds()
	// 上游与文件上传已完成后，数据库侧的附件、用量和完成态仍需保持原子一致。
	if reuseUserMessage {
		if err = s.repo.CompleteAssistantMessageWithGeneratedAttachments(ctx,
			assistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:      "image",
				Content:          content,
				InputTokens:      usage.InputTokens,
				OutputTokens:     usage.OutputTokens,
				CacheReadTokens:  usage.CacheReadTokens,
				CacheWriteTokens: usage.CacheWriteTokens,
				ReasoningTokens:  usage.ReasoningTokens,
				LatencyMS:        latencyMS,
				Status:           "success",
			},
			attachmentRows,
		); err != nil {
			retErr = err
			return nil, err
		}
	} else {
		if err = s.repo.CompleteAssistantMessageWithAttachments(ctx,
			userMessage.ID,
			repository.MessageUsageUpdate{
				InputTokens:      usage.InputTokens,
				CacheReadTokens:  usage.CacheReadTokens,
				CacheWriteTokens: usage.CacheWriteTokens,
			},
			assistantMessage.ID,
			repository.AssistantMessageCompletionUpdate{
				ContentType:     "image",
				Content:         content,
				OutputTokens:    usage.OutputTokens,
				ReasoningTokens: usage.ReasoningTokens,
				LatencyMS:       latencyMS,
				Status:          "success",
			},
			attachmentRows,
		); err != nil {
			retErr = err
			return nil, err
		}
	}
	assistantMessage.Content = content
	assistantMessage.OutputTokens = usage.OutputTokens
	assistantMessage.ReasoningTokens = usage.ReasoningTokens
	assistantMessage.TokenUsage = assistantMessage.OutputTokens + assistantMessage.ReasoningTokens
	if reuseUserMessage {
		assistantMessage.TokenUsage += usage.InputTokens + usage.CacheReadTokens + usage.CacheWriteTokens
	}
	assistantMessage.LatencyMS = latencyMS
	assistantMessage.Status = "success"
	assistantMessage.Attachments = string(marshalAttachmentSnapshots(attachmentsFromFiles(uploaded)))
	run.InputTokens = usage.InputTokens
	run.OutputTokens = usage.OutputTokens
	run.CacheReadTokens = usage.CacheReadTokens
	run.CacheWriteTokens = usage.CacheWriteTokens
	run.ReasoningTokens = usage.ReasoningTokens

	return &SendMessageResult{
		UserMessage:         *userMessage,
		AssistantMessage:    *assistantMessage,
		MetadataRefreshHint: conversationMetadataRefreshHint(*conversation, *userMessage),
		UpstreamID:          route.UpstreamID,
		UpstreamName:        route.UpstreamName,
		PlatformModelName:   route.PlatformModelName,
		RoutedBindingCode:   route.BindingCode,
		UpstreamModelName:   route.UpstreamModel,
		UpstreamProtocol:    route.Protocol,
		EffectiveOptions:    filteredOptions,
		UsageSpeed:          usage.Speed,
		UsageServiceTier:    usage.ServiceTier,
		RawUsageJSON:        usage.RawUsageJSON,
		CacheWrite5mTokens:  usage.CacheWrite5mTokens,
		CacheWrite1hTokens:  usage.CacheWrite1hTokens,
		LatencyMS:           latencyMS,
		StartedAt:           startedAt,
	}, nil
}

func mediaUserContentType(taskType MediaImageTaskType, hasAttachments bool) string {
	if taskType == MediaImageTaskEdit || hasAttachments {
		return "mixed"
	}
	return "text"
}

func mediaImageRunningMessage(taskType MediaImageTaskType) string {
	if taskType == MediaImageTaskEdit {
		return "editing image"
	}
	if taskType == MediaVideoTaskGeneration {
		return "generating video"
	}
	return "generating image"
}

func mediaQueuedMessage(taskType MediaImageTaskType) string {
	if taskType == MediaVideoTaskGeneration {
		return "video task queued"
	}
	return "image task queued"
}

func mediaAssistantContentType(taskType MediaImageTaskType) string {
	if taskType == MediaVideoTaskGeneration {
		return "video"
	}
	return "image"
}

// resolveMediaImageEditInputs 读取图片编辑输入图，确保只有图片文件进入图片编辑协议。
func (s *Service) resolveMediaImageEditInputs(ctx context.Context, input MediaImageInput) ([]AttachmentInput, []llm.ContentPart, error) {
	if input.TaskType != MediaImageTaskEdit {
		return nil, nil, nil
	}
	attachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(attachments) == 0 || len(attachments) > maxMediaImageEditInputImages {
		if len(attachments) == 0 {
			return nil, nil, ErrMediaImageEditInputRequired
		}
		return nil, nil, ErrMediaImageEditTooManyInputs
	}
	parts := make([]llm.ContentPart, 0, len(attachments))
	for _, attachment := range attachments {
		if normalizeAttachmentKind(attachment.Kind, attachment.MimeType) != "image" {
			return nil, nil, ErrMediaImageEditInputInvalid
		}
		part, readErr := s.readMediaImageEditFile(ctx, input.UserID, attachment.FileID)
		if readErr != nil {
			return nil, nil, readErr
		}
		part.FileName = mediaImageEditInputFileName(attachment.FileName, part.MimeType)
		parts = append(parts, part)
	}
	return attachments, parts, nil
}

func (s *Service) resolveMediaImageEditMask(ctx context.Context, userID uint, fileID string) (*llm.ContentPart, error) {
	if strings.TrimSpace(fileID) == "" {
		return nil, nil
	}
	part, err := s.readMediaImageEditFile(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	return &part, nil
}

func (s *Service) readMediaImageEditFile(ctx context.Context, userID uint, fileID string) (llm.ContentPart, error) {
	content, err := s.OpenFileContent(ctx, userID, strings.TrimSpace(fileID))
	if err != nil {
		return llm.ContentPart{}, err
	}
	defer content.Reader.Close() //nolint:errcheck

	limit := s.cfg.Snapshot().MaxUploadFileBytes
	if limit <= 0 {
		limit = 20 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(content.Reader, limit+1))
	if err != nil {
		return llm.ContentPart{}, err
	}
	if int64(len(data)) > limit {
		return llm.ContentPart{}, ErrFileTooLarge
	}
	mimeType := strings.TrimSpace(content.ContentType)
	if mimeType == "" {
		mimeType = strings.TrimSpace(content.File.DetectedMIME)
	}
	data, mimeType, err = normalizeMediaImageEditInput(data, mimeType)
	if err != nil {
		return llm.ContentPart{}, ErrMediaImageEditInputInvalid
	}
	return llm.ContentPart{
		Kind:     llm.ContentPartImage,
		MimeType: mimeType,
		Data:     data,
		FileName: mediaImageEditInputFileName(content.File.FileName, mimeType),
	}, nil
}

func (s *Service) resolveMediaVideoReferenceInputs(
	ctx context.Context,
	input MediaImageInput,
	modelName string,
	options map[string]interface{},
) ([]AttachmentInput, []llm.ContentPart, error) {
	if input.TaskType != MediaVideoTaskGeneration {
		return nil, nil, nil
	}
	attachments, err := s.resolveAttachments(ctx, input.UserID, input.FileIDs)
	if err != nil {
		return nil, nil, err
	}
	if len(attachments) == 0 {
		return nil, nil, nil
	}
	if len(attachments) > 1 {
		return nil, nil, ErrMediaVideoTooManyReferenceImages
	}
	attachment := attachments[0]
	if normalizeAttachmentKind(attachment.Kind, attachment.MimeType) != "image" {
		return nil, nil, ErrMediaVideoReferenceImageInvalid
	}
	part, err := s.readMediaVideoReferenceImageFile(ctx, input.UserID, attachment.FileID)
	if err != nil {
		return nil, nil, err
	}
	part.FileName = strings.TrimSpace(attachment.FileName)
	targetSize := resolveMediaVideoSize(modelName, options)
	width, height, err := imageDimensionsFromBytes(part.Data)
	if err != nil {
		return nil, nil, ErrMediaVideoReferenceImageInvalid
	}
	if fmt.Sprintf("%dx%d", width, height) != targetSize {
		return nil, nil, ErrMediaVideoReferenceSizeMismatch
	}
	return attachments, []llm.ContentPart{part}, nil
}

func (s *Service) readMediaVideoReferenceImageFile(ctx context.Context, userID uint, fileID string) (llm.ContentPart, error) {
	content, err := s.OpenFileContent(ctx, userID, strings.TrimSpace(fileID))
	if err != nil {
		return llm.ContentPart{}, err
	}
	defer content.Reader.Close() //nolint:errcheck

	limit := s.cfg.Snapshot().MaxUploadFileBytes
	if limit <= 0 {
		limit = 20 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(content.Reader, limit+1))
	if err != nil {
		return llm.ContentPart{}, err
	}
	if int64(len(data)) > limit {
		return llm.ContentPart{}, ErrFileTooLarge
	}
	mimeType := strings.TrimSpace(content.ContentType)
	if mimeType == "" {
		mimeType = strings.TrimSpace(content.File.DetectedMIME)
	}
	data, mimeType, err = validateMediaVideoReferenceImageBytes(data, mimeType)
	if err != nil {
		return llm.ContentPart{}, ErrMediaVideoReferenceImageInvalid
	}
	return llm.ContentPart{
		Kind:     llm.ContentPartImage,
		MimeType: mimeType,
		Data:     data,
		FileName: strings.TrimSpace(content.File.FileName),
	}, nil
}

func sanitizeOpenAIVideoGenerationOptions(modelName string, options map[string]interface{}) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}
	seconds := strings.TrimSpace(modelOptionStringValue(options["seconds"]))
	switch seconds {
	case "4", "8", "12":
		options["seconds"] = seconds
	default:
		delete(options, "seconds")
	}
	size := strings.TrimSpace(modelOptionStringValue(options["size"]))
	if !isMediaVideoSizeAllowed(modelName, size) {
		delete(options, "size")
	} else {
		options["size"] = size
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func mediaVideoBillingDurationSeconds(options map[string]interface{}) int64 {
	switch strings.TrimSpace(modelOptionStringValue(options["seconds"])) {
	case "8":
		return 8
	case "12":
		return 12
	default:
		return 4
	}
}

func resolveMediaVideoSize(modelName string, options map[string]interface{}) string {
	size := strings.TrimSpace(modelOptionStringValue(options["size"]))
	if isMediaVideoSizeAllowed(modelName, size) {
		return size
	}
	return "1280x720"
}

func isMediaVideoSizeAllowed(modelName string, value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range mediaVideoSizeValues(modelName) {
		if value == item {
			return true
		}
	}
	return false
}

func mediaVideoSizeValues(modelName string) []string {
	if strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "sora-2-pro") {
		return []string{"720x1280", "1280x720", "1024x1792", "1792x1024", "1080x1920", "1920x1080"}
	}
	return []string{"720x1280", "1280x720"}
}

func validateMediaVideoReferenceImageBytes(data []byte, declaredMIME string) ([]byte, string, error) {
	detected := detectGeneratedImageMIME(data)
	switch detected {
	case "image/jpeg", "image/png", "image/webp":
		return data, detected, nil
	default:
		return nil, strings.TrimSpace(declaredMIME), fmt.Errorf("video reference image content is not a supported image")
	}
}

func imageDimensionsFromBytes(data []byte) (int, int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// emitMediaEvent 输出媒体任务状态事件；失败不影响主流程。
func emitMediaEvent(onEvent func(string, map[string]interface{}) error, status string, message string, contentType ...string) {
	if onEvent == nil {
		return
	}
	payload := map[string]interface{}{
		"status":  status,
		"message": message,
	}
	if len(contentType) > 0 && strings.TrimSpace(contentType[0]) != "" {
		payload["content_type"] = strings.TrimSpace(contentType[0])
	}
	_ = onEvent("media_status", payload)
}

func emitMediaImageDelta(onEvent func(string, map[string]interface{}) error, event llm.GenerateStreamEvent) error {
	if onEvent == nil || event.GeneratedImage == nil {
		return nil
	}
	image := event.GeneratedImage
	if strings.TrimSpace(image.B64JSON) == "" {
		return nil
	}
	return onEvent("media_image_delta", map[string]interface{}{
		"index":          event.GeneratedImageIndex,
		"b64_json":       image.B64JSON,
		"mime_type":      strings.TrimSpace(image.MIMEType),
		"revised_prompt": strings.TrimSpace(image.RevisedPrompt),
	})
}

func emitMediaVideoStatus(onEvent func(string, map[string]interface{}) error, status *llm.GeneratedVideoStatus) error {
	if onEvent == nil || status == nil {
		return nil
	}
	upstreamStatus := strings.TrimSpace(status.Status)
	normalized := strings.ToLower(upstreamStatus)
	appStatus := "running"
	message := mediaImageRunningMessage(MediaVideoTaskGeneration)
	if normalized == "queued" {
		appStatus = "queued"
		message = mediaQueuedMessage(MediaVideoTaskGeneration)
	}
	payload := map[string]interface{}{
		"status":          appStatus,
		"message":         message,
		"video_id":        strings.TrimSpace(status.ID),
		"upstream_status": upstreamStatus,
	}
	if status.Progress != nil {
		payload["progress"] = clampPercent(*status.Progress)
	}
	if size := strings.TrimSpace(status.Size); size != "" {
		payload["size"] = size
	}
	if seconds := strings.TrimSpace(status.Seconds); seconds != "" {
		payload["seconds"] = seconds
	}
	return onEvent("media_status", payload)
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func mediaImageStreamEnabled(protocol string, upstreamModel string, capabilitiesJSON string) bool {
	return llm.SupportsImageGenerationStream(protocol, upstreamModel) && !mediaImageStreamExplicitlyDisabled(capabilitiesJSON)
}

func withGeminiInteractionResponseType(options map[string]interface{}, responseType string) map[string]interface{} {
	next := make(map[string]interface{}, len(options)+1)
	for key, value := range options {
		next[key] = value
	}
	format := map[string]interface{}{}
	if raw, ok := next["response_format"].(map[string]interface{}); ok {
		for key, value := range raw {
			format[key] = value
		}
	}
	if raw, ok := next["responseFormat"].(map[string]interface{}); ok {
		for key, value := range raw {
			format[key] = value
		}
		delete(next, "responseFormat")
	}
	format["type"] = strings.TrimSpace(responseType)
	next["response_format"] = format
	return next
}

func mediaImageStreamExplicitlyDisabled(capabilitiesJSON string) bool {
	raw := strings.TrimSpace(capabilitiesJSON)
	if raw == "" {
		return false
	}
	var caps mediaImageCapabilities
	if err := json.Unmarshal([]byte(raw), &caps); err != nil {
		return false
	}
	return caps.Image.Stream != nil && !*caps.Image.Stream
}

// readGeneratedImage 读取上游图片结果，并统一校验为可保存的图片字节。
// 上游临时 URL 只用于服务端下载，最终不会直接写入消息内容，避免长期依赖外部地址。
func (s *Service) readGeneratedImage(ctx context.Context, image llm.GeneratedImage) ([]byte, string, error) {
	mimeType := strings.TrimSpace(image.MIMEType)
	if mimeType == "" {
		mimeType = "image/png"
	}
	if b64 := strings.TrimSpace(image.B64JSON); b64 != "" {
		data, err := base64.StdEncoding.DecodeString(stripBase64DataURLPrefix(b64))
		if err != nil {
			return nil, mimeType, err
		}
		return validateGeneratedImageBytes(data, mimeType)
	}
	url := strings.TrimSpace(image.URL)
	if url == "" {
		return nil, mimeType, ErrUpstreamEmptyResponse
	}
	if isGeminiFilesURL(url) {
		return nil, mimeType, fmt.Errorf("Gemini Files generated image URI is not supported")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, mimeType, err
	}
	cfg := s.cfg.Snapshot()
	client := security.NewOutboundHTTPClient(cfg.Env, cfg.SSRFProtectionEnabled, 60*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, mimeType, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mimeType, fmt.Errorf("download generated image failed: HTTP %d", resp.StatusCode)
	}
	if contentType := strings.TrimSpace(resp.Header.Get("Content-Type")); strings.HasPrefix(strings.ToLower(contentType), "image/") {
		mimeType = strings.Split(contentType, ";")[0]
	}
	limit := cfg.MaxUploadFileBytes
	if limit <= 0 {
		limit = 20 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, mimeType, err
	}
	if int64(len(data)) > limit {
		return nil, mimeType, ErrFileTooLarge
	}
	return validateGeneratedImageBytes(data, mimeType)
}

// stripBase64DataURLPrefix 兼容 data URL 和纯 base64 两种上游返回格式。
func stripBase64DataURLPrefix(value string) string {
	normalized := strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(normalized), "data:") {
		return normalized
	}
	if index := strings.Index(normalized, ","); index >= 0 {
		return strings.TrimSpace(normalized[index+1:])
	}
	return normalized
}

// validateGeneratedImageBytes 使用文件头重新识别 MIME，防止把非图片响应保存成图片文件。
func validateGeneratedImageBytes(data []byte, declaredMIME string) ([]byte, string, error) {
	detected := detectGeneratedImageMIME(data)
	if detected == "" {
		return nil, strings.TrimSpace(declaredMIME), fmt.Errorf("generated image content is not a supported image")
	}
	return data, detected, nil
}

// detectGeneratedImageMIME 识别当前支持落库的图片格式。
func detectGeneratedImageMIME(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")) {
		return "image/webp"
	}
	if len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return "image/gif"
	}
	return ""
}

// imageFileExtension 根据最终识别出的 MIME 决定生成文件扩展名。
func imageFileExtension(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

// generatedImageFileName 使用模型名和生成时间构造稳定可读的文件名。
func generatedImageFileName(modelName string, capturedAt time.Time, index int, total int, mimeType string) string {
	base := sanitizeGeneratedImageFileBase(modelName)
	timestamp := fmt.Sprintf("%s-%03d", capturedAt.Format("20060102-150405"), capturedAt.Nanosecond()/int(time.Millisecond))
	if total > 1 {
		return fmt.Sprintf("%s-%s-%02d%s", base, timestamp, index+1, imageFileExtension(mimeType))
	}
	return fmt.Sprintf("%s-%s%s", base, timestamp, imageFileExtension(mimeType))
}

// sanitizeGeneratedImageFileBase 清理模型名，确保生成文件名不含路径分隔符或不可控字符。
func sanitizeGeneratedImageFileBase(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "image"
	}
	var builder strings.Builder
	builder.Grow(len(normalized))
	lastDash := false
	for _, item := range normalized {
		allowed := (item >= 'a' && item <= 'z') ||
			(item >= 'A' && item <= 'Z') ||
			(item >= '0' && item <= '9') ||
			item == '.' ||
			item == '_' ||
			item == '-'
		if allowed {
			builder.WriteRune(item)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), ".-_")
	if result == "" {
		return "image"
	}
	if len(result) > 80 {
		result = strings.Trim(result[:80], ".-_")
	}
	if result == "" {
		return "image"
	}
	return result
}

// generatedImageMarkdown 将已保存的文件对象转换为受保护文件接口的 markdown 引用。
func generatedImageMarkdown(files []model.FileObject) string {
	blocks := make([]string, 0, len(files))
	for i, file := range files {
		alt := "Generated image"
		if len(files) > 1 {
			alt = fmt.Sprintf("Generated image %d", i+1)
		}
		blocks = append(blocks, fmt.Sprintf("![%s](/api/v1/files/%s/content)", alt, file.FileID))
	}
	return strings.Join(blocks, "\n\n")
}

type mediaVideoCompletionInput struct {
	Input            MediaImageInput
	Conversation     *model.Conversation
	UserMessage      *model.Message
	AssistantMessage *model.Message
	Route            *channel.ResolvedRoute
	FilteredOptions  map[string]interface{}
	Output           *llm.GenerateOutput
	StartedAt        time.Time
}

func (s *Service) completeMediaVideoGeneration(ctx context.Context, input mediaVideoCompletionInput) (*SendMessageResult, error) {
	emitMediaEvent(input.Input.OnEvent, "saving_artifact", "saving video")
	uploaded, attachmentRows, err := s.saveGeneratedVideos(ctx, assistantGeneratedVideoSaveInput{
		UserID:         input.Input.UserID,
		ConversationID: input.Input.ConversationID,
		MessageID:      input.AssistantMessage.ID,
		ModelName:      input.Route.PlatformModelName,
		Videos:         input.Output.GeneratedVideos,
	})
	if err != nil {
		_ = s.repo.UpdateMessageState(ctx, input.AssistantMessage.ID, "error", classifyRunErrorCode(err), truncateError(messageErrorSummary(err), 255))
		return nil, err
	}
	usage := input.Output.Usage
	input.UserMessage.InputTokens = usage.InputTokens
	input.UserMessage.CacheReadTokens = usage.CacheReadTokens
	input.UserMessage.CacheWriteTokens = usage.CacheWriteTokens
	input.UserMessage.TokenUsage = usage.InputTokens + usage.CacheReadTokens + usage.CacheWriteTokens

	content := generatedVideoMarkdown(uploaded)
	latencyMS := time.Since(input.StartedAt).Milliseconds()
	if err = s.repo.CompleteAssistantMessageWithAttachments(ctx,
		input.UserMessage.ID,
		repository.MessageUsageUpdate{
			InputTokens:      usage.InputTokens,
			CacheReadTokens:  usage.CacheReadTokens,
			CacheWriteTokens: usage.CacheWriteTokens,
		},
		input.AssistantMessage.ID,
		repository.AssistantMessageCompletionUpdate{
			Content:         content,
			OutputTokens:    usage.OutputTokens,
			ReasoningTokens: usage.ReasoningTokens,
			LatencyMS:       latencyMS,
			Status:          "success",
		},
		attachmentRows,
	); err != nil {
		return nil, err
	}
	input.AssistantMessage.Content = content
	input.AssistantMessage.OutputTokens = usage.OutputTokens
	input.AssistantMessage.ReasoningTokens = usage.ReasoningTokens
	input.AssistantMessage.TokenUsage = input.AssistantMessage.OutputTokens + input.AssistantMessage.ReasoningTokens
	input.AssistantMessage.LatencyMS = latencyMS
	input.AssistantMessage.Status = "success"
	input.AssistantMessage.Attachments = string(marshalAttachmentSnapshots(videoAttachmentsFromFiles(uploaded)))
	s.maybeGenerateConversationMetadataAsync(*input.Conversation, *input.UserMessage)

	return &SendMessageResult{
		UserMessage:        *input.UserMessage,
		AssistantMessage:   *input.AssistantMessage,
		UpstreamID:         input.Route.UpstreamID,
		UpstreamName:       input.Route.UpstreamName,
		PlatformModelName:  input.Route.PlatformModelName,
		RoutedBindingCode:  input.Route.BindingCode,
		UpstreamModelName:  input.Route.UpstreamModel,
		UpstreamProtocol:   input.Route.Protocol,
		EffectiveOptions:   input.FilteredOptions,
		UsageSpeed:         usage.Speed,
		UsageServiceTier:   usage.ServiceTier,
		CacheWrite5mTokens: usage.CacheWrite5mTokens,
		CacheWrite1hTokens: usage.CacheWrite1hTokens,
		DurationSeconds:    mediaVideoBillingDurationSeconds(input.FilteredOptions),
		LatencyMS:          latencyMS,
	}, nil
}

type assistantGeneratedVideoSaveInput struct {
	UserID         uint
	ConversationID uint
	MessageID      uint
	ModelName      string
	Videos         []llm.GeneratedVideo
}

func (s *Service) saveGeneratedVideos(ctx context.Context, input assistantGeneratedVideoSaveInput) ([]model.FileObject, []model.Attachment, error) {
	if len(input.Videos) == 0 {
		return nil, nil, nil
	}
	uploaded := make([]model.FileObject, 0, len(input.Videos))
	attachmentRows := make([]model.Attachment, 0, len(input.Videos))
	now := time.Now()
	for i, video := range input.Videos {
		data, mimeType, err := validateGeneratedVideoBytes(video.Data, video.MIMEType)
		if err != nil {
			return nil, nil, err
		}
		fileName := generatedVideoFileName(input.ModelName, now, i, len(input.Videos), mimeType)
		uploadResult, err := s.UploadFile(ctx, appupload.UploadFileInput{
			UserID:       input.UserID,
			Purpose:      "generated_video",
			FileName:     fileName,
			MimeType:     mimeType,
			DeclaredSize: int64(len(data)),
			Reader:       bytes.NewReader(data),
		})
		if err != nil {
			return nil, nil, err
		}
		file := uploadResult.File
		uploaded = append(uploaded, file)
		attachmentRows = append(attachmentRows, model.Attachment{
			ConversationID: input.ConversationID,
			MessageID:      input.MessageID,
			UserID:         input.UserID,
			FileID:         file.FileID,
			Kind:           "video",
			FileName:       file.FileName,
			MimeType:       file.DetectedMIME,
			FileSize:       file.SizeBytes,
			SHA256:         file.SHA256,
			StoragePath:    file.StoragePath,
			Status:         "active",
			UploadedAt:     now,
		})
	}
	return uploaded, attachmentRows, nil
}

func generatedVideoFileName(modelName string, capturedAt time.Time, index int, total int, mimeType string) string {
	base := sanitizeGeneratedImageFileBase(modelName)
	if base == "image" {
		base = "video"
	}
	timestamp := fmt.Sprintf("%s-%03d", capturedAt.Format("20060102-150405"), capturedAt.Nanosecond()/int(time.Millisecond))
	if total > 1 {
		return fmt.Sprintf("%s-%s-%02d%s", base, timestamp, index+1, videoFileExtension(mimeType))
	}
	return fmt.Sprintf("%s-%s%s", base, timestamp, videoFileExtension(mimeType))
}

func videoFileExtension(mimeType string) string {
	if strings.EqualFold(strings.TrimSpace(mimeType), "video/mp4") {
		return ".mp4"
	}
	return ".mp4"
}

func generatedVideoMarkdown(files []model.FileObject) string {
	blocks := make([]string, 0, len(files))
	for i, file := range files {
		label := "Generated video"
		if len(files) > 1 {
			label = fmt.Sprintf("Generated video %d", i+1)
		}
		blocks = append(blocks, fmt.Sprintf("[%s](/api/v1/files/%s/content)", label, file.FileID))
	}
	return strings.Join(blocks, "\n\n")
}

func videoAttachmentsFromFiles(files []model.FileObject) []AttachmentInput {
	items := make([]AttachmentInput, 0, len(files))
	for _, file := range files {
		items = append(items, AttachmentInput{
			FileObjID:        file.ID,
			FileID:           file.FileID,
			Kind:             "video",
			FileName:         file.FileName,
			MimeType:         file.MimeType,
			DetectedMIME:     file.DetectedMIME,
			FileCategory:     file.FileCategory,
			FileSize:         file.SizeBytes,
			SHA256:           file.SHA256,
			StoragePath:      file.StoragePath,
			ProcessingStatus: file.ProcessingStatus,
			ProcessingReady:  file.ProcessingReady,
		})
	}
	return items
}

// attachmentsFromFiles 生成消息附件快照，供流式完成事件立即返回给前端。
func attachmentsFromFiles(files []model.FileObject) []AttachmentInput {
	items := make([]AttachmentInput, 0, len(files))
	for _, file := range files {
		items = append(items, AttachmentInput{
			FileObjID:        file.ID,
			FileID:           file.FileID,
			Kind:             "image",
			FileName:         file.FileName,
			MimeType:         file.MimeType,
			DetectedMIME:     file.DetectedMIME,
			FileCategory:     file.FileCategory,
			FileSize:         file.SizeBytes,
			SHA256:           file.SHA256,
			StoragePath:      file.StoragePath,
			ProcessingStatus: file.ProcessingStatus,
			ProcessingReady:  file.ProcessingReady,
		})
	}
	return items
}
