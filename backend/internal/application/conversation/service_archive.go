package conversation

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/google/uuid"
)

const (
	ConversationArchiveSchema = "deeix-chat.conversation.v1"

	maxConversationArchiveMessages        = 5000
	maxConversationArchiveRuns            = 5000
	maxConversationArchiveAttachments     = 10000
	maxConversationArchiveStringChars     = 1_000_000
	maxConversationArchiveMetadataChars   = 20000
	conversationArchiveImportPageSize     = 500
	conversationArchiveAttachmentFilePref = "metadata_"
)

// ConversationArchive 是单条会话 JSON 备份格式。
type ConversationArchive struct {
	Schema       string
	ExportedAt   time.Time
	Conversation ConversationArchiveMetadata
	Runs         []ConversationArchiveRun
	Messages     []ConversationArchiveMessage
}

// ConversationArchiveMetadata 保存可恢复的会话元信息。
type ConversationArchiveMetadata struct {
	OriginalPublicID    string
	OriginalProjectName string
	Title               string
	LabelsJSON          string
	Model               string
	Provider            string
	IsStarred           bool
	Status              string
	ContextPolicyJSON   string
	MessageCount        int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ConversationArchiveRun 保存消息展示所需的运行快照。
type ConversationArchiveRun struct {
	OriginalRunID       string
	TaskType            string
	Endpoint            string
	Provider            string
	ProviderProtocol    string
	RequestedModelName  string
	PlatformModelName   string
	ModelVendor         string
	ModelIcon           string
	UpstreamModelName   string
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheWriteTokens    int64
	ReasoningTokens     int64
	ToolCallsCount      int
	FirstTokenLatencyMS int64
	TotalLatencyMS      int64
	Status              string
	ErrorCode           string
	ErrorMessage        string
	StartedAt           time.Time
	EndedAt             *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// ConversationArchiveMessage 保存单条消息及其可展示附属信息。
type ConversationArchiveMessage struct {
	OriginalPublicID string
	ParentPublicID   string
	SourcePublicID   string
	RunID            string
	Role             string
	ContentType      string
	Content          string
	BranchReason     string
	TokenUsage       int64
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
	LatencyMS        int64
	Status           string
	ErrorCode        string
	ErrorMessage     string
	Attachments      []ConversationArchiveAttachment
	ProcessTrace     *ConversationArchiveProcessTrace
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ConversationArchiveAttachment 是附件的元数据快照，不包含文件内容和原始 fileID。
type ConversationArchiveAttachment struct {
	Kind                   string
	FileName               string
	MimeType               string
	DetectedMIME           string
	FileCategory           string
	FileSize               int64
	ProcessingStatus       string
	ProcessingReady        bool
	ProcessingErrorCode    string
	ProcessingErrorMessage string
}

type ConversationArchiveProcessTrace struct {
	Enabled       bool
	Status        string
	Process       *ConversationArchiveTraceBlock
	Tools         *ConversationArchiveTraceBlock
	UpstreamThink *ConversationArchiveTraceBlock
	PromptTrace   *ConversationArchivePromptTrace
	Events        []ConversationArchiveTraceEvent
}

type ConversationArchiveTraceBlock struct {
	Title           string
	Summary         string
	ContentMarkdown string
	Status          string
	Stage           string
	RoundID         string
	ParentEventID   string
	UpdatedAt       time.Time
	PayloadJSON     string
}

type ConversationArchiveTraceEvent struct {
	EventID         string
	EventType       string
	Phase           string
	Stage           string
	RoundID         string
	ParentEventID   string
	Title           string
	Summary         string
	ContentMarkdown string
	Status          string
	Seq             int
	StartedAt       time.Time
	EndedAt         *time.Time
	UpdatedAt       time.Time
	PayloadJSON     string
}

type ConversationArchivePromptTrace struct {
	Mode                   string
	PromptFingerprint      string
	StatefulUsed           bool
	StatefulDisabledReason string
	TotalTokenEstimate     int64
	SentTokenEstimate      int64
	FullMessageCount       int
	SentMessageCount       int
	StatefulSavedMessages  int
	StatefulSavedTokens    int64
	Blocks                 []ConversationArchivePromptBlock
}

type ConversationArchivePromptBlock struct {
	Kind          string
	Title         string
	TokenEstimate int64
	Cacheable     bool
	SourceCount   int
	SourceRefs    []ConversationArchivePromptSourceRef
}

type ConversationArchivePromptSourceRef struct {
	SourceType string
	SourceID   string
	Title      string
}

// ExportConversationArchive 导出当前用户的一条会话为 JSON 归档对象。
func (s *Service) ExportConversationArchive(ctx context.Context, userID uint, publicID string) (*ConversationArchive, error) {
	conversation, err := s.GetConversationByPublicID(ctx, userID, publicID)
	if err != nil {
		return nil, err
	}
	messages, err := s.listAllArchiveMessages(ctx, conversation.ID)
	if err != nil {
		return nil, err
	}
	if err = s.hydrateMessageProcessTracesForArchive(ctx, messages); err != nil {
		return nil, err
	}
	runs, err := s.archiveRunsForMessages(ctx, userID, conversation.ID, messages)
	if err != nil {
		return nil, err
	}
	return &ConversationArchive{
		Schema:     ConversationArchiveSchema,
		ExportedAt: time.Now().UTC(),
		Conversation: ConversationArchiveMetadata{
			OriginalPublicID:    conversation.PublicID,
			OriginalProjectName: conversation.ProjectName,
			Title:               conversation.Title,
			LabelsJSON:          normalizeArchiveLabelsJSON(conversation.LabelsJSON),
			Model:               conversation.Model,
			Provider:            conversation.Provider,
			IsStarred:           conversation.IsStarred,
			Status:              normalizeArchiveConversationStatus(conversation.Status),
			ContextPolicyJSON:   conversation.ContextPolicy,
			MessageCount:        len(messages),
			CreatedAt:           conversation.CreatedAt,
			UpdatedAt:           conversation.UpdatedAt,
		},
		Runs:     archiveRunsFromDomain(runs),
		Messages: archiveMessagesFromDomain(messages),
	}, nil
}

// ImportConversationArchive 导入单条会话归档，并始终创建新会话副本。
func (s *Service) ImportConversationArchive(ctx context.Context, userID uint, archive *ConversationArchive) (*model.Conversation, error) {
	if err := validateConversationArchive(archive); err != nil {
		return nil, err
	}
	var imported *model.Conversation
	err := s.repo.WithConversationTransaction(ctx, func(repo repository.ConversationRepository) error {
		created, err := s.createConversationFromArchive(ctx, repo, userID, archive)
		if err != nil {
			return err
		}
		imported = created
		return nil
	})
	if err != nil {
		return nil, err
	}
	return imported, nil
}

func (s *Service) listAllArchiveMessages(ctx context.Context, conversationID uint) ([]model.Message, error) {
	result := make([]model.Message, 0)
	offset := 0
	for {
		items, total, err := s.repo.ListMessages(ctx, conversationID, offset, conversationArchiveImportPageSize)
		if err != nil {
			return nil, err
		}
		if len(result)+len(items) > maxConversationArchiveMessages {
			return nil, ErrConversationArchiveTooLarge
		}
		result = append(result, items...)
		if len(items) == 0 || int64(len(result)) >= total {
			break
		}
		offset += len(items)
	}
	return result, nil
}

func (s *Service) hydrateMessageProcessTracesForArchive(ctx context.Context, items []model.Message) error {
	messageIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item.Role == "assistant" && item.ID != 0 {
			messageIDs = append(messageIDs, item.ID)
		}
	}
	if len(messageIDs) == 0 {
		return nil
	}
	rows, err := s.repo.ListConversationMessageTracesByMessageIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	eventRows, err := s.repo.ListConversationMessageTraceEventsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return err
	}
	byMessageID := make(map[uint][]model.MessageTrace, len(messageIDs))
	for _, row := range rows {
		byMessageID[row.MessageID] = append(byMessageID[row.MessageID], row)
	}
	eventsByMessageID := make(map[uint][]model.MessageTraceEventRow, len(messageIDs))
	for _, row := range eventRows {
		eventsByMessageID[row.MessageID] = append(eventsByMessageID[row.MessageID], row)
	}
	for i := range items {
		if items[i].Role == "assistant" {
			items[i].ProcessTrace = buildMessageProcessTraceDTO(byMessageID[items[i].ID], eventsByMessageID[items[i].ID])
		}
	}
	return nil
}

func (s *Service) archiveRunsForMessages(ctx context.Context, userID uint, conversationID uint, messages []model.Message) ([]model.Run, error) {
	runIDs := collectMessageRunIDsForArchive(messages)
	if len(runIDs) == 0 {
		return nil, nil
	}
	runs, err := s.repo.ListConversationRunsByRunIDs(ctx, userID, conversationID, runIDs)
	if err != nil {
		return nil, err
	}
	byRunID := make(map[string]model.Run, len(runs))
	for _, run := range runs {
		if runID := strings.TrimSpace(run.RunID); runID != "" {
			byRunID[runID] = run
		}
	}
	ordered := make([]model.Run, 0, len(runs))
	for _, runID := range runIDs {
		if run, ok := byRunID[runID]; ok {
			ordered = append(ordered, run)
		}
	}
	return ordered, nil
}

func collectMessageRunIDsForArchive(messages []model.Message) []string {
	seen := make(map[string]struct{}, len(messages))
	result := make([]string, 0, len(messages))
	for _, message := range messages {
		runID := strings.TrimSpace(message.RunID)
		if runID == "" {
			continue
		}
		if _, ok := seen[runID]; ok {
			continue
		}
		seen[runID] = struct{}{}
		result = append(result, runID)
	}
	return result
}

func archiveRunsFromDomain(runs []model.Run) []ConversationArchiveRun {
	result := make([]ConversationArchiveRun, 0, len(runs))
	for _, run := range runs {
		result = append(result, ConversationArchiveRun{
			OriginalRunID:       run.RunID,
			TaskType:            run.TaskType,
			Endpoint:            run.Endpoint,
			Provider:            run.Provider,
			ProviderProtocol:    run.ProviderProtocol,
			RequestedModelName:  run.RequestedModelName,
			PlatformModelName:   run.PlatformModelName,
			ModelVendor:         run.ModelVendor,
			ModelIcon:           run.ModelIcon,
			UpstreamModelName:   run.UpstreamModelName,
			InputTokens:         run.InputTokens,
			OutputTokens:        run.OutputTokens,
			CacheReadTokens:     run.CacheReadTokens,
			CacheWriteTokens:    run.CacheWriteTokens,
			ReasoningTokens:     run.ReasoningTokens,
			ToolCallsCount:      run.ToolCallsCount,
			FirstTokenLatencyMS: run.FirstTokenLatencyMS,
			TotalLatencyMS:      run.TotalLatencyMS,
			Status:              run.Status,
			ErrorCode:           run.ErrorCode,
			ErrorMessage:        run.ErrorMessage,
			StartedAt:           run.StartedAt,
			EndedAt:             run.EndedAt,
			CreatedAt:           run.CreatedAt,
			UpdatedAt:           run.UpdatedAt,
		})
	}
	return result
}

func archiveMessagesFromDomain(messages []model.Message) []ConversationArchiveMessage {
	result := make([]ConversationArchiveMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, ConversationArchiveMessage{
			OriginalPublicID: strings.TrimSpace(message.PublicID),
			ParentPublicID:   strings.TrimSpace(message.ParentPublicID),
			SourcePublicID:   strings.TrimSpace(message.SourcePublicID),
			RunID:            strings.TrimSpace(message.RunID),
			Role:             strings.TrimSpace(message.Role),
			ContentType:      strings.TrimSpace(message.ContentType),
			Content:          message.Content,
			BranchReason:     strings.TrimSpace(message.BranchReason),
			TokenUsage:       message.TokenUsage,
			InputTokens:      message.InputTokens,
			OutputTokens:     message.OutputTokens,
			CacheReadTokens:  message.CacheReadTokens,
			CacheWriteTokens: message.CacheWriteTokens,
			ReasoningTokens:  message.ReasoningTokens,
			LatencyMS:        message.LatencyMS,
			Status:           strings.TrimSpace(message.Status),
			ErrorCode:        message.ErrorCode,
			ErrorMessage:     message.ErrorMessage,
			Attachments:      archiveAttachmentsFromRaw(message.Attachments),
			ProcessTrace:     archiveProcessTraceFromDomain(message.ProcessTrace),
			CreatedAt:        message.CreatedAt,
			UpdatedAt:        message.UpdatedAt,
		})
	}
	return result
}

func archiveAttachmentsFromRaw(raw string) []ConversationArchiveAttachment {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	items := []struct {
		Kind                   string `json:"kind"`
		FileName               string `json:"file_name"`
		MimeType               string `json:"mime_type"`
		DetectedMIME           string `json:"detected_mime"`
		FileCategory           string `json:"file_category"`
		FileSize               int64  `json:"file_size"`
		ProcessingStatus       string `json:"processing_status"`
		ProcessingReady        bool   `json:"processing_ready"`
		ProcessingErrorCode    string `json:"processing_error_code"`
		ProcessingErrorMessage string `json:"processing_error_message"`
	}{}
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return nil
	}
	result := make([]ConversationArchiveAttachment, 0, len(items))
	for _, item := range items {
		fileName := strings.TrimSpace(item.FileName)
		if fileName == "" {
			continue
		}
		result = append(result, ConversationArchiveAttachment{
			Kind:                   normalizeArchiveAttachmentKind(item.Kind),
			FileName:               fileName,
			MimeType:               strings.TrimSpace(item.MimeType),
			DetectedMIME:           strings.TrimSpace(item.DetectedMIME),
			FileCategory:           strings.TrimSpace(item.FileCategory),
			FileSize:               item.FileSize,
			ProcessingStatus:       strings.TrimSpace(item.ProcessingStatus),
			ProcessingReady:        item.ProcessingReady,
			ProcessingErrorCode:    strings.TrimSpace(item.ProcessingErrorCode),
			ProcessingErrorMessage: strings.TrimSpace(item.ProcessingErrorMessage),
		})
	}
	return result
}

func archiveProcessTraceFromDomain(trace *model.MessageProcessTrace) *ConversationArchiveProcessTrace {
	if trace == nil {
		return nil
	}
	return &ConversationArchiveProcessTrace{
		Enabled:       trace.Enabled,
		Status:        trace.Status,
		Process:       archiveTraceBlockFromDomain(trace.Process),
		Tools:         archiveTraceBlockFromDomain(trace.Tools),
		UpstreamThink: archiveTraceBlockFromDomain(trace.UpstreamThink),
		PromptTrace:   archivePromptTraceFromDomain(trace.PromptTrace),
		Events:        archiveTraceEventsFromDomain(trace.Events),
	}
}

func archiveTraceBlockFromDomain(block *model.MessageTraceBlock) *ConversationArchiveTraceBlock {
	if block == nil {
		return nil
	}
	return &ConversationArchiveTraceBlock{
		Title:           block.Title,
		Summary:         block.Summary,
		ContentMarkdown: block.ContentMarkdown,
		Status:          block.Status,
		Stage:           block.Stage,
		RoundID:         block.RoundID,
		ParentEventID:   block.ParentEventID,
		UpdatedAt:       block.UpdatedAt,
		PayloadJSON:     sanitizeSharedTracePayloadJSON(block.PayloadJSON),
	}
}

func archiveTraceEventsFromDomain(events []model.MessageTraceEvent) []ConversationArchiveTraceEvent {
	if len(events) == 0 {
		return nil
	}
	result := make([]ConversationArchiveTraceEvent, 0, len(events))
	for _, event := range events {
		result = append(result, ConversationArchiveTraceEvent{
			EventID:         event.EventID,
			EventType:       event.EventType,
			Phase:           event.Phase,
			Stage:           event.Stage,
			RoundID:         event.RoundID,
			ParentEventID:   event.ParentEventID,
			Title:           event.Title,
			Summary:         event.Summary,
			ContentMarkdown: event.ContentMarkdown,
			Status:          event.Status,
			Seq:             event.Seq,
			StartedAt:       event.StartedAt,
			EndedAt:         event.EndedAt,
			UpdatedAt:       event.UpdatedAt,
			PayloadJSON:     sanitizeSharedTracePayloadJSON(event.PayloadJSON),
		})
	}
	return result
}

func archivePromptTraceFromDomain(trace *model.MessagePromptTrace) *ConversationArchivePromptTrace {
	if trace == nil {
		return nil
	}
	blocks := make([]ConversationArchivePromptBlock, 0, len(trace.Blocks))
	for _, block := range trace.Blocks {
		refs := make([]ConversationArchivePromptSourceRef, 0, len(block.SourceRefs))
		for _, ref := range block.SourceRefs {
			refs = append(refs, ConversationArchivePromptSourceRef{
				SourceType: ref.SourceType,
				SourceID:   ref.SourceID,
				Title:      ref.Title,
			})
		}
		blocks = append(blocks, ConversationArchivePromptBlock{
			Kind:          block.Kind,
			Title:         block.Title,
			TokenEstimate: block.TokenEstimate,
			Cacheable:     block.Cacheable,
			SourceCount:   block.SourceCount,
			SourceRefs:    refs,
		})
	}
	return &ConversationArchivePromptTrace{
		Mode:                   trace.Mode,
		PromptFingerprint:      trace.PromptFingerprint,
		StatefulUsed:           trace.StatefulUsed,
		StatefulDisabledReason: trace.StatefulDisabledReason,
		TotalTokenEstimate:     trace.TotalTokenEstimate,
		SentTokenEstimate:      trace.SentTokenEstimate,
		FullMessageCount:       trace.FullMessageCount,
		SentMessageCount:       trace.SentMessageCount,
		StatefulSavedMessages:  trace.StatefulSavedMessages,
		StatefulSavedTokens:    trace.StatefulSavedTokens,
		Blocks:                 blocks,
	}
}

func validateConversationArchive(archive *ConversationArchive) error {
	if archive == nil || strings.TrimSpace(archive.Schema) != ConversationArchiveSchema {
		return ErrInvalidConversationArchive
	}
	if len(archive.Messages) == 0 {
		return ErrInvalidConversationArchive
	}
	if len(archive.Messages) > maxConversationArchiveMessages || len(archive.Runs) > maxConversationArchiveRuns {
		return ErrConversationArchiveTooLarge
	}
	if !validArchiveMetadataString(archive.Conversation.Title, 255) ||
		!validArchiveMetadataString(archive.Conversation.Model, 128) ||
		!validArchiveMetadataString(archive.Conversation.Provider, 32) ||
		!validArchiveMetadataString(archive.Conversation.LabelsJSON, maxConversationArchiveMetadataChars) ||
		!validArchiveMetadataString(archive.Conversation.ContextPolicyJSON, maxConversationArchiveMetadataChars) {
		return ErrInvalidConversationArchive
	}
	if strings.TrimSpace(archive.Conversation.LabelsJSON) != "" && !json.Valid([]byte(strings.TrimSpace(archive.Conversation.LabelsJSON))) {
		return ErrInvalidConversationArchive
	}
	if strings.TrimSpace(archive.Conversation.ContextPolicyJSON) != "" && !json.Valid([]byte(strings.TrimSpace(archive.Conversation.ContextPolicyJSON))) {
		return ErrInvalidConversationArchive
	}
	runIDs := make(map[string]struct{}, len(archive.Runs))
	for _, run := range archive.Runs {
		runID := strings.TrimSpace(run.OriginalRunID)
		if runID == "" || !validArchiveMetadataString(runID, 64) {
			return ErrInvalidConversationArchive
		}
		if _, exists := runIDs[runID]; exists {
			return ErrInvalidConversationArchive
		}
		runIDs[runID] = struct{}{}
	}
	seenMessages := make(map[string]struct{}, len(archive.Messages))
	attachmentCount := 0
	for _, message := range archive.Messages {
		messageID := strings.TrimSpace(message.OriginalPublicID)
		if messageID == "" || !validArchiveMetadataString(messageID, 64) {
			return ErrInvalidConversationArchive
		}
		if _, exists := seenMessages[messageID]; exists {
			return ErrInvalidConversationArchive
		}
		if !validArchiveRole(message.Role) || !validArchiveContentType(message.ContentType) || !validArchiveBranchReason(message.BranchReason) {
			return ErrInvalidConversationArchive
		}
		if !validArchiveContentString(message.Content) ||
			!validArchiveMetadataString(message.Status, 32) ||
			!validArchiveMetadataString(message.ErrorCode, 64) ||
			!validArchiveMetadataString(message.ErrorMessage, 255) {
			return ErrInvalidConversationArchive
		}
		if runID := strings.TrimSpace(message.RunID); runID != "" {
			if _, ok := runIDs[runID]; !ok {
				return ErrInvalidConversationArchive
			}
		}
		if ref := strings.TrimSpace(message.ParentPublicID); ref != "" {
			if _, ok := seenMessages[ref]; !ok {
				return ErrInvalidConversationArchive
			}
		}
		if ref := strings.TrimSpace(message.SourcePublicID); ref != "" {
			if _, ok := seenMessages[ref]; !ok {
				return ErrInvalidConversationArchive
			}
		}
		for _, attachment := range message.Attachments {
			attachmentCount++
			if attachmentCount > maxConversationArchiveAttachments || !validArchiveAttachment(attachment) {
				return ErrInvalidConversationArchive
			}
		}
		if tracePayloadHasSensitiveField(message.ProcessTrace) {
			return ErrInvalidConversationArchive
		}
		seenMessages[messageID] = struct{}{}
	}
	return nil
}

func (s *Service) createConversationFromArchive(
	ctx context.Context,
	repo repository.ConversationRepository,
	userID uint,
	archive *ConversationArchive,
) (*model.Conversation, error) {
	now := time.Now().UTC()
	title := strings.TrimSpace(archive.Conversation.Title)
	if title == "" {
		title = "导入的会话"
	}
	platformModel := strings.TrimSpace(archive.Conversation.Model)
	status := normalizeArchiveConversationStatus(archive.Conversation.Status)
	var starredAt *time.Time
	if archive.Conversation.IsStarred {
		starredAt = &now
	}
	contextPolicy := strings.TrimSpace(archive.Conversation.ContextPolicyJSON)
	if contextPolicy == "" {
		contextPolicy = buildContextPolicyJSON(s.cfg.Snapshot())
	}
	target := &model.Conversation{
		UserID:          userID,
		PublicID:        normalizePublicID(uuid.NewString()),
		Title:           title,
		LabelsJSON:      normalizeArchiveLabelsJSON(archive.Conversation.LabelsJSON),
		Model:           platformModel,
		Provider:        firstNonEmptyString(strings.TrimSpace(archive.Conversation.Provider), inferProvider(platformModel)),
		SessionKey:      uuid.NewString(),
		IsStarred:       archive.Conversation.IsStarred,
		StarredAt:       starredAt,
		MessageCount:    0,
		Status:          status,
		ContextPolicy:   contextPolicy,
		LastCompactedAt: nil,
		LastResponseID:  "",
	}
	if err := repo.CreateConversation(ctx, target); err != nil {
		return nil, err
	}
	runIDMap, err := createArchiveRuns(ctx, repo, userID, target.ID, archive.Runs)
	if err != nil {
		return nil, err
	}
	messageIDMap := make(map[string]uint, len(archive.Messages))
	for _, archivedMessage := range archive.Messages {
		newRunID := runIDMap[strings.TrimSpace(archivedMessage.RunID)]
		if newRunID == "" && archivedMessage.ProcessTrace != nil {
			newRunID = "run_" + normalizePublicID(uuid.NewString())
		}
		message, err := createArchiveMessage(ctx, repo, userID, target.ID, archivedMessage, newRunID, messageIDMap)
		if err != nil {
			return nil, err
		}
		messageIDMap[strings.TrimSpace(archivedMessage.OriginalPublicID)] = message.ID
		if err = createArchiveMessageAttachments(ctx, repo, userID, target.ID, message.ID, archivedMessage.Attachments, now); err != nil {
			return nil, err
		}
		if err = createArchiveMessageTrace(ctx, repo, userID, target.ID, message.ID, newRunID, archivedMessage.ProcessTrace); err != nil {
			return nil, err
		}
	}
	if err := repo.IncrementMessageCount(ctx, target.ID, len(archive.Messages)); err != nil {
		return nil, err
	}
	target.MessageCount = len(archive.Messages)
	return target, nil
}

func createArchiveRuns(
	ctx context.Context,
	repo repository.ConversationRepository,
	userID uint,
	conversationID uint,
	runs []ConversationArchiveRun,
) (map[string]string, error) {
	result := make(map[string]string, len(runs))
	for _, archivedRun := range runs {
		originalRunID := strings.TrimSpace(archivedRun.OriginalRunID)
		if originalRunID == "" {
			continue
		}
		newRunID := "run_" + normalizePublicID(uuid.NewString())
		startedAt := archivedRun.StartedAt
		if startedAt.IsZero() {
			startedAt = time.Now().UTC()
		}
		run := &model.Run{
			RunID:               newRunID,
			RequestID:           normalizePublicID(uuid.NewString()),
			UserID:              userID,
			ConversationID:      conversationID,
			TaskType:            strings.TrimSpace(archivedRun.TaskType),
			Endpoint:            strings.TrimSpace(archivedRun.Endpoint),
			Provider:            strings.TrimSpace(archivedRun.Provider),
			ProviderProtocol:    strings.TrimSpace(archivedRun.ProviderProtocol),
			RequestedModelName:  strings.TrimSpace(archivedRun.RequestedModelName),
			PlatformModelName:   strings.TrimSpace(archivedRun.PlatformModelName),
			ModelVendor:         strings.TrimSpace(archivedRun.ModelVendor),
			ModelIcon:           strings.TrimSpace(archivedRun.ModelIcon),
			UpstreamModelName:   strings.TrimSpace(archivedRun.UpstreamModelName),
			InputTokens:         archivedRun.InputTokens,
			OutputTokens:        archivedRun.OutputTokens,
			CacheReadTokens:     archivedRun.CacheReadTokens,
			CacheWriteTokens:    archivedRun.CacheWriteTokens,
			ReasoningTokens:     archivedRun.ReasoningTokens,
			ToolCallsCount:      archivedRun.ToolCallsCount,
			FirstTokenLatencyMS: archivedRun.FirstTokenLatencyMS,
			TotalLatencyMS:      archivedRun.TotalLatencyMS,
			Status:              strings.TrimSpace(archivedRun.Status),
			ErrorCode:           strings.TrimSpace(archivedRun.ErrorCode),
			ErrorMessage:        strings.TrimSpace(archivedRun.ErrorMessage),
			StartedAt:           startedAt,
			EndedAt:             archivedRun.EndedAt,
		}
		if err := repo.CreateConversationRun(ctx, run); err != nil {
			return nil, err
		}
		result[originalRunID] = newRunID
	}
	return result, nil
}

func createArchiveMessage(
	ctx context.Context,
	repo repository.ConversationRepository,
	userID uint,
	conversationID uint,
	archivedMessage ConversationArchiveMessage,
	runID string,
	messageIDMap map[string]uint,
) (*model.Message, error) {
	var parentMessageID *uint
	if parentPublicID := strings.TrimSpace(archivedMessage.ParentPublicID); parentPublicID != "" {
		value := messageIDMap[parentPublicID]
		parentMessageID = &value
	}
	var sourceMessageID *uint
	if sourcePublicID := strings.TrimSpace(archivedMessage.SourcePublicID); sourcePublicID != "" {
		value := messageIDMap[sourcePublicID]
		sourceMessageID = &value
	}
	message := &model.Message{
		ConversationID:   conversationID,
		UserID:           userID,
		PublicID:         normalizePublicID(uuid.NewString()),
		ParentMessageID:  parentMessageID,
		RunID:            strings.TrimSpace(runID),
		Role:             normalizeArchiveRole(archivedMessage.Role),
		ContentType:      normalizeArchiveContentType(archivedMessage.ContentType),
		Content:          archivedMessage.Content,
		BranchReason:     normalizeArchiveBranchReason(archivedMessage.BranchReason),
		SourceMessageID:  sourceMessageID,
		TokenUsage:       archivedMessage.TokenUsage,
		InputTokens:      archivedMessage.InputTokens,
		OutputTokens:     archivedMessage.OutputTokens,
		CacheReadTokens:  archivedMessage.CacheReadTokens,
		CacheWriteTokens: archivedMessage.CacheWriteTokens,
		ReasoningTokens:  archivedMessage.ReasoningTokens,
		LatencyMS:        archivedMessage.LatencyMS,
		BilledCurrency:   "USD",
		BilledNanousd:    0,
		PricingSnapshot:  "",
		Status:           normalizeArchiveMessageStatus(archivedMessage.Status),
		ErrorCode:        strings.TrimSpace(archivedMessage.ErrorCode),
		ErrorMessage:     strings.TrimSpace(archivedMessage.ErrorMessage),
	}
	if err := repo.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	return message, nil
}

func createArchiveMessageAttachments(
	ctx context.Context,
	repo repository.ConversationRepository,
	userID uint,
	conversationID uint,
	messageID uint,
	attachments []ConversationArchiveAttachment,
	now time.Time,
) error {
	if len(attachments) == 0 {
		return nil
	}
	items := make([]model.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		items = append(items, model.Attachment{
			ConversationID: conversationID,
			MessageID:      messageID,
			UserID:         userID,
			FileID:         conversationArchiveAttachmentFilePref + normalizePublicID(uuid.NewString()),
			Kind:           normalizeArchiveAttachmentKind(attachment.Kind),
			FileName:       strings.TrimSpace(attachment.FileName),
			MimeType:       strings.TrimSpace(attachment.MimeType),
			FileSize:       attachment.FileSize,
			Status:         "metadata_only",
			MetaJSON:       metadataOnlyAttachmentJSON(attachment),
			UploadedAt:     now,
		})
	}
	return repo.CreateAttachments(ctx, items)
}

func createArchiveMessageTrace(
	ctx context.Context,
	repo repository.ConversationRepository,
	userID uint,
	conversationID uint,
	messageID uint,
	runID string,
	trace *ConversationArchiveProcessTrace,
) error {
	if trace == nil {
		return nil
	}
	startedAt := time.Now().UTC()
	if err := createArchiveTraceBlock(ctx, repo, userID, conversationID, messageID, runID, messageTraceTypeProcess, 1, startedAt, trace.Process); err != nil {
		return err
	}
	if err := createArchiveTraceBlock(ctx, repo, userID, conversationID, messageID, runID, messageTraceTypeTools, 2, startedAt, trace.Tools); err != nil {
		return err
	}
	if err := createArchiveTraceBlock(ctx, repo, userID, conversationID, messageID, runID, messageTraceTypeUpstreamThink, 3, startedAt, trace.UpstreamThink); err != nil {
		return err
	}
	for _, event := range trace.Events {
		eventID := strings.TrimSpace(event.EventID)
		if eventID == "" {
			eventID = "event_" + normalizePublicID(uuid.NewString())
		}
		eventStartedAt := event.StartedAt
		if eventStartedAt.IsZero() {
			eventStartedAt = startedAt
		}
		row := &model.MessageTraceEventRow{
			MessageID:       messageID,
			ConversationID:  conversationID,
			UserID:          userID,
			RunID:           strings.TrimSpace(runID),
			EventID:         eventID,
			EventType:       strings.TrimSpace(event.EventType),
			Phase:           strings.TrimSpace(event.Phase),
			Stage:           strings.TrimSpace(event.Stage),
			RoundID:         strings.TrimSpace(event.RoundID),
			ParentEventID:   strings.TrimSpace(event.ParentEventID),
			Status:          strings.TrimSpace(event.Status),
			Title:           strings.TrimSpace(event.Title),
			Summary:         strings.TrimSpace(event.Summary),
			ContentMarkdown: event.ContentMarkdown,
			PayloadJSON:     sanitizeSharedTracePayloadJSON(event.PayloadJSON),
			Seq:             event.Seq,
			StartedAt:       eventStartedAt,
			EndedAt:         event.EndedAt,
		}
		if err := repo.UpsertConversationMessageTraceEvent(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func createArchiveTraceBlock(
	ctx context.Context,
	repo repository.ConversationRepository,
	userID uint,
	conversationID uint,
	messageID uint,
	runID string,
	traceType string,
	seq int,
	startedAt time.Time,
	block *ConversationArchiveTraceBlock,
) error {
	if block == nil {
		return nil
	}
	rowStartedAt := block.UpdatedAt
	if rowStartedAt.IsZero() {
		rowStartedAt = startedAt
	}
	return repo.UpsertConversationMessageTrace(ctx, &model.MessageTrace{
		MessageID:       messageID,
		ConversationID:  conversationID,
		UserID:          userID,
		RunID:           strings.TrimSpace(runID),
		TraceType:       traceType,
		Status:          strings.TrimSpace(block.Status),
		Stage:           strings.TrimSpace(block.Stage),
		RoundID:         strings.TrimSpace(block.RoundID),
		ParentEventID:   strings.TrimSpace(block.ParentEventID),
		Title:           strings.TrimSpace(block.Title),
		Summary:         strings.TrimSpace(block.Summary),
		ContentMarkdown: block.ContentMarkdown,
		PayloadJSON:     sanitizeSharedTracePayloadJSON(block.PayloadJSON),
		Seq:             seq,
		StartedAt:       rowStartedAt,
	})
}

func metadataOnlyAttachmentJSON(attachment ConversationArchiveAttachment) string {
	payload := map[string]interface{}{
		"metadata_only":            true,
		"detected_mime":            strings.TrimSpace(attachment.DetectedMIME),
		"file_category":            strings.TrimSpace(attachment.FileCategory),
		"processing_status":        strings.TrimSpace(attachment.ProcessingStatus),
		"processing_ready":         attachment.ProcessingReady,
		"processing_error_code":    strings.TrimSpace(attachment.ProcessingErrorCode),
		"processing_error_message": strings.TrimSpace(attachment.ProcessingErrorMessage),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return `{"metadata_only":true}`
	}
	return string(data)
}

func normalizeArchiveLabelsJSON(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" || value == "null" || !json.Valid([]byte(value)) {
		return "[]"
	}
	return value
}

func normalizeArchiveConversationStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "archived":
		return "archived"
	default:
		return "active"
	}
}

func normalizeArchiveRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "assistant":
		return "assistant"
	case "system":
		return "system"
	case "tool":
		return "tool"
	default:
		return "user"
	}
}

func normalizeArchiveContentType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "markdown":
		return "markdown"
	case "image":
		return "image"
	case "file":
		return "file"
	case "mixed":
		return "mixed"
	default:
		return "text"
	}
}

func normalizeArchiveBranchReason(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "retry":
		return "retry"
	case "edit":
		return "edit"
	default:
		return "default"
	}
}

func normalizeArchiveMessageStatus(value string) string {
	if status := strings.TrimSpace(value); status != "" {
		return status
	}
	return "success"
}

func normalizeArchiveAttachmentKind(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "image") {
		return "image"
	}
	return "file"
}

func validArchiveRole(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "user", "assistant", "system", "tool":
		return true
	default:
		return false
	}
}

func validArchiveContentType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "text", "markdown", "image", "file", "mixed":
		return true
	default:
		return false
	}
}

func validArchiveBranchReason(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "default", "retry", "edit":
		return true
	default:
		return false
	}
}

func validArchiveAttachment(attachment ConversationArchiveAttachment) bool {
	kind := strings.ToLower(strings.TrimSpace(attachment.Kind))
	return validArchiveMetadataString(attachment.Kind, 32) &&
		(kind == "file" || kind == "image") &&
		strings.TrimSpace(attachment.FileName) != "" &&
		validArchiveMetadataString(attachment.FileName, 255) &&
		validArchiveMetadataString(attachment.MimeType, 128) &&
		validArchiveMetadataString(attachment.DetectedMIME, 128) &&
		validArchiveMetadataString(attachment.FileCategory, 32) &&
		validArchiveMetadataString(attachment.ProcessingStatus, 32) &&
		validArchiveMetadataString(attachment.ProcessingErrorCode, 64) &&
		validArchiveMetadataString(attachment.ProcessingErrorMessage, 255) &&
		attachment.FileSize >= 0
}

func validArchiveContentString(value string) bool {
	return len([]rune(value)) <= maxConversationArchiveStringChars
}

func validArchiveMetadataString(value string, maxChars int) bool {
	return len([]rune(strings.TrimSpace(value))) <= maxChars
}

func tracePayloadHasSensitiveField(trace *ConversationArchiveProcessTrace) bool {
	if trace == nil {
		return false
	}
	for _, payload := range []string{
		payloadFromArchiveTraceBlock(trace.Process),
		payloadFromArchiveTraceBlock(trace.Tools),
		payloadFromArchiveTraceBlock(trace.UpstreamThink),
	} {
		if archivePayloadContainsSensitiveField(payload) {
			return true
		}
	}
	for _, event := range trace.Events {
		if archivePayloadContainsSensitiveField(event.PayloadJSON) {
			return true
		}
	}
	return false
}

func payloadFromArchiveTraceBlock(block *ConversationArchiveTraceBlock) string {
	if block == nil {
		return ""
	}
	return block.PayloadJSON
}

func archivePayloadContainsSensitiveField(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return false
	}
	return archivePayloadMapContainsSensitiveField(payload, "")
}

func archivePayloadMapContainsSensitiveField(payload map[string]interface{}, parentKey string) bool {
	for key, value := range payload {
		if isSharedTraceInternalField(key, parentKey) {
			return true
		}
		switch child := value.(type) {
		case map[string]interface{}:
			if archivePayloadMapContainsSensitiveField(child, key) {
				return true
			}
		case []interface{}:
			for _, item := range child {
				itemMap, ok := item.(map[string]interface{})
				if ok && archivePayloadMapContainsSensitiveField(itemMap, key) {
					return true
				}
			}
		}
	}
	return false
}
