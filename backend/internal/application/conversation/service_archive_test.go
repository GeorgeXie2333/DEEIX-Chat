package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestExportConversationArchiveSanitizesTraceAndAttachmentMetadata(t *testing.T) {
	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	repo := &archiveRepoFake{
		conversations: map[string]*model.Conversation{
			"conv_source": {
				ID:           10,
				UserID:       7,
				PublicID:     "conv_source",
				Title:        "研究记录",
				LabelsJSON:   `["backup"]`,
				Model:        "gpt-test",
				Provider:     "openai",
				SessionKey:   "session_should_not_export",
				Status:       "active",
				CreatedAt:    now,
				UpdatedAt:    now,
				MessageCount: 3,
			},
		},
		messages: []model.Message{
			{
				ID:             1,
				ConversationID: 10,
				UserID:         7,
				PublicID:       "msg_user",
				Role:           "user",
				ContentType:    "text",
				Content:        "请分析附件",
				Status:         "success",
				Attachments: `[{
					"kind":"file",
					"file_id":"file_original_secret",
					"file_name":"report.pdf",
					"mime_type":"application/pdf",
					"detected_mime":"application/pdf",
					"file_category":"document",
					"file_size":128,
					"processing_status":"ready",
					"processing_ready":true
				}]`,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:               2,
				ConversationID:   10,
				UserID:           7,
				PublicID:         "msg_assistant",
				ParentPublicID:   "msg_user",
				RunID:            "run_source",
				Role:             "assistant",
				ContentType:      "markdown",
				Content:          "结论",
				InputTokens:      10,
				OutputTokens:     20,
				CacheReadTokens:  1,
				CacheWriteTokens: 2,
				ReasoningTokens:  3,
				Status:           "success",
				CreatedAt:        now,
				UpdatedAt:        now,
			},
			{
				ID:             3,
				ConversationID: 10,
				UserID:         7,
				PublicID:       "msg_branch",
				ParentPublicID: "msg_user",
				SourcePublicID: "msg_assistant",
				RunID:          "run_source",
				Role:           "assistant",
				ContentType:    "markdown",
				Content:        "分支结论",
				BranchReason:   "retry",
				Status:         "success",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
		runs: []model.Run{{
			RunID:              "run_source",
			UserID:             7,
			ConversationID:     10,
			RequestedModelName: "gpt-test",
			Status:             "completed",
			StartedAt:          now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}},
		traces: []model.MessageTrace{{
			MessageID:       2,
			ConversationID:  10,
			UserID:          7,
			RunID:           "run_source",
			TraceType:       messageTraceTypeProcess,
			Status:          messageTraceStatusCompleted,
			Title:           "处理过程",
			ContentMarkdown: "可展示内容",
			PayloadJSON:     `{"visible":"ok","api_key":"secret","nested":{"authorization":"bearer","keep":"yes"},"upstream":{"name":"hidden","safe":"yes"}}`,
			Seq:             1,
			StartedAt:       now,
			UpdatedAt:       now,
		}},
		traceEvents: []model.MessageTraceEventRow{{
			MessageID:      2,
			ConversationID: 10,
			UserID:         7,
			RunID:          "run_source",
			EventID:        "event_source",
			EventType:      "step",
			Phase:          messageTraceTypeProcess,
			Status:         messageTraceStatusCompleted,
			Title:          "事件",
			PayloadJSON:    `{"result":"ok","secret_key":"hidden"}`,
			Seq:            1,
			StartedAt:      now,
			UpdatedAt:      now,
		}},
	}
	svc := &Service{repo: repo, cfg: config.NewRuntime(config.Config{})}

	archive, err := svc.ExportConversationArchive(context.Background(), 7, "conv_source")
	if err != nil {
		t.Fatalf("export archive: %v", err)
	}
	if archive.Schema != ConversationArchiveSchema {
		t.Fatalf("unexpected schema: %s", archive.Schema)
	}
	if archive.Conversation.OriginalPublicID != "conv_source" || archive.Conversation.MessageCount != 3 {
		t.Fatalf("unexpected conversation metadata: %#v", archive.Conversation)
	}
	if len(archive.Messages) != 3 {
		t.Fatalf("expected all branch messages, got %d", len(archive.Messages))
	}
	if archive.Messages[1].ParentPublicID != "msg_user" || archive.Messages[2].SourcePublicID != "msg_assistant" {
		t.Fatalf("message references were not preserved: %#v", archive.Messages)
	}
	if len(archive.Runs) != 1 || archive.Runs[0].OriginalRunID != "run_source" {
		t.Fatalf("unexpected runs: %#v", archive.Runs)
	}
	if len(archive.Messages[0].Attachments) != 1 {
		t.Fatalf("expected attachment metadata, got %#v", archive.Messages[0].Attachments)
	}
	attachmentJSON, _ := json.Marshal(archive.Messages[0].Attachments[0])
	if strings.Contains(string(attachmentJSON), "file_original_secret") || strings.Contains(string(attachmentJSON), "file_id") {
		t.Fatalf("attachment archive leaked original file id: %s", attachmentJSON)
	}
	processPayload := archive.Messages[1].ProcessTrace.Process.PayloadJSON
	if processPayload == "" || strings.Contains(processPayload, "api_key") || strings.Contains(processPayload, "authorization") || strings.Contains(processPayload, "hidden") {
		t.Fatalf("process payload was not sanitized: %s", processPayload)
	}
	if !strings.Contains(processPayload, "visible") || !strings.Contains(processPayload, "keep") {
		t.Fatalf("process payload lost safe fields: %s", processPayload)
	}
	eventPayload := archive.Messages[1].ProcessTrace.Events[0].PayloadJSON
	if strings.Contains(eventPayload, "secret_key") || !strings.Contains(eventPayload, "result") {
		t.Fatalf("event payload was not sanitized: %s", eventPayload)
	}

	if _, err := svc.ExportConversationArchive(context.Background(), 8, "conv_source"); !errors.Is(err, ErrConversationNotFound) {
		t.Fatalf("expected owner check failure, got %v", err)
	}
}

func TestListAllArchiveMessagesRejectsOverLimitPageBeforeAppending(t *testing.T) {
	for _, count := range []int{maxConversationArchiveMessages + 1, maxConversationArchiveMessages + 500} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			messages := make([]model.Message, count)
			for index := range messages {
				messages[index].ConversationID = 10
			}
			service := &Service{repo: &archiveRepoFake{messages: messages}}

			items, err := service.listAllArchiveMessages(context.Background(), 10)
			if !errors.Is(err, ErrConversationArchiveTooLarge) {
				t.Fatalf("listAllArchiveMessages() error = %v, want ErrConversationArchiveTooLarge", err)
			}
			if items != nil {
				t.Fatalf("listAllArchiveMessages() items = %d, want nil on overflow", len(items))
			}
		})
	}
}

func TestImportConversationArchiveCreatesIndependentMetadataOnlyCopy(t *testing.T) {
	now := time.Date(2026, 5, 27, 11, 0, 0, 0, time.UTC)
	archive := &ConversationArchive{
		Schema: ConversationArchiveSchema,
		Conversation: ConversationArchiveMetadata{
			OriginalPublicID:  "conv_old",
			Title:             "导入源",
			LabelsJSON:        `["x"]`,
			Model:             "gpt-test",
			Provider:          "openai",
			Status:            "active",
			ContextPolicyJSON: `{}`,
		},
		Runs: []ConversationArchiveRun{{
			OriginalRunID:      "run_old",
			TaskType:           "chat",
			Provider:           "openai",
			RequestedModelName: "gpt-test",
			Status:             "completed",
			StartedAt:          now,
		}},
		Messages: []ConversationArchiveMessage{
			{
				OriginalPublicID: "msg_old_user",
				Role:             "user",
				ContentType:      "text",
				Content:          "hello",
				Status:           "success",
				Attachments: []ConversationArchiveAttachment{{
					Kind:                "file",
					FileName:            "backup.txt",
					MimeType:            "text/plain",
					DetectedMIME:        "text/plain",
					FileCategory:        "document",
					FileSize:            12,
					ProcessingStatus:    "ready",
					ProcessingReady:     true,
					ProcessingErrorCode: "",
				}},
			},
			{
				OriginalPublicID: "msg_old_assistant",
				ParentPublicID:   "msg_old_user",
				RunID:            "run_old",
				Role:             "assistant",
				ContentType:      "markdown",
				Content:          "answer",
				TokenUsage:       30,
				InputTokens:      10,
				OutputTokens:     20,
				Status:           "success",
				ProcessTrace: &ConversationArchiveProcessTrace{
					Process: &ConversationArchiveTraceBlock{
						Title:       "过程",
						Status:      messageTraceStatusCompleted,
						PayloadJSON: `{"safe":"yes"}`,
						UpdatedAt:   now,
					},
					Events: []ConversationArchiveTraceEvent{{
						EventID:     "event_old",
						EventType:   "step",
						Phase:       messageTraceTypeProcess,
						Status:      messageTraceStatusCompleted,
						Title:       "事件",
						PayloadJSON: `{"safe":"event"}`,
						Seq:         1,
						StartedAt:   now,
						UpdatedAt:   now,
					}},
				},
			},
			{
				OriginalPublicID: "msg_old_branch",
				ParentPublicID:   "msg_old_user",
				SourcePublicID:   "msg_old_assistant",
				RunID:            "run_old",
				Role:             "assistant",
				ContentType:      "markdown",
				Content:          "branch",
				BranchReason:     "retry",
				Status:           "success",
			},
		},
	}
	repo := &archiveRepoFake{}
	svc := &Service{repo: repo, cfg: config.NewRuntime(config.Config{})}

	imported, err := svc.ImportConversationArchive(context.Background(), 11, archive)
	if err != nil {
		t.Fatalf("import archive: %v", err)
	}
	if imported.UserID != 11 || imported.PublicID == "conv_old" || imported.SessionKey == "" {
		t.Fatalf("conversation was not recreated independently: %#v", imported)
	}
	if imported.MessageCount != 3 || repo.incrementDelta != 3 {
		t.Fatalf("message count was not updated: imported=%d increment=%d", imported.MessageCount, repo.incrementDelta)
	}
	if len(repo.createdRuns) != 1 {
		t.Fatalf("expected one recreated run, got %#v", repo.createdRuns)
	}
	newRunID := repo.createdRuns[0].RunID
	if newRunID == "" || newRunID == "run_old" || repo.createdRuns[0].RequestID == "" ||
		repo.createdRuns[0].UpstreamID != 0 || repo.createdRuns[0].UpstreamName != "" || repo.createdRuns[0].RoutedBindingCode != "" {
		t.Fatalf("run was not sanitized and remapped: %#v", repo.createdRuns[0])
	}
	if len(repo.createdMessages) != 3 {
		t.Fatalf("expected three recreated messages, got %#v", repo.createdMessages)
	}
	if repo.createdMessages[0].PublicID == "msg_old_user" || repo.createdMessages[1].PublicID == "msg_old_assistant" {
		t.Fatalf("message public ids were reused: %#v", repo.createdMessages)
	}
	if repo.createdMessages[1].ParentMessageID == nil || *repo.createdMessages[1].ParentMessageID != repo.createdMessages[0].ID {
		t.Fatalf("assistant parent was not remapped: %#v", repo.createdMessages[1])
	}
	if repo.createdMessages[2].SourceMessageID == nil || *repo.createdMessages[2].SourceMessageID != repo.createdMessages[1].ID {
		t.Fatalf("branch source was not remapped: %#v", repo.createdMessages[2])
	}
	if repo.createdMessages[1].RunID != newRunID || repo.createdMessages[2].RunID != newRunID {
		t.Fatalf("message run ids were not remapped: %#v", repo.createdMessages)
	}
	for _, message := range repo.createdMessages {
		if message.BilledNanousd != 0 || message.PricingSnapshot != "" {
			t.Fatalf("imported message should not carry billing state: %#v", message)
		}
	}
	if len(repo.createdAttachments) != 1 {
		t.Fatalf("expected one metadata-only attachment, got %#v", repo.createdAttachments)
	}
	attachment := repo.createdAttachments[0]
	if attachment.Status != "metadata_only" || !strings.HasPrefix(attachment.FileID, conversationArchiveAttachmentFilePref) ||
		attachment.StoragePath != "" || attachment.SHA256 != "" {
		t.Fatalf("attachment was not metadata-only: %#v", attachment)
	}
	if !strings.Contains(attachment.MetaJSON, `"metadata_only":true`) || !strings.Contains(attachment.MetaJSON, `"detected_mime":"text/plain"`) {
		t.Fatalf("attachment metadata was not preserved: %s", attachment.MetaJSON)
	}
	if len(repo.upsertedTraces) != 1 || repo.upsertedTraces[0].MessageID != repo.createdMessages[1].ID || repo.upsertedTraces[0].RunID != newRunID {
		t.Fatalf("trace block was not recreated against remapped ids: %#v", repo.upsertedTraces)
	}
	if len(repo.upsertedTraceEvents) != 1 || repo.upsertedTraceEvents[0].MessageID != repo.createdMessages[1].ID || repo.upsertedTraceEvents[0].RunID != newRunID {
		t.Fatalf("trace event was not recreated against remapped ids: %#v", repo.upsertedTraceEvents)
	}
}

func TestValidateConversationArchiveRejectsInvalidInput(t *testing.T) {
	base := func() *ConversationArchive {
		return &ConversationArchive{
			Schema: ConversationArchiveSchema,
			Conversation: ConversationArchiveMetadata{
				Title:      "valid",
				LabelsJSON: "[]",
				Model:      "gpt-test",
				Provider:   "openai",
				Status:     "active",
			},
			Runs: []ConversationArchiveRun{{
				OriginalRunID: "run_1",
				StartedAt:     time.Now().UTC(),
			}},
			Messages: []ConversationArchiveMessage{{
				OriginalPublicID: "msg_1",
				RunID:            "run_1",
				Role:             "user",
				ContentType:      "text",
				Content:          "hello",
				Status:           "success",
			}},
		}
	}
	tests := []struct {
		name string
		edit func(*ConversationArchive)
		want error
	}{
		{
			name: "wrong schema",
			edit: func(archive *ConversationArchive) {
				archive.Schema = "deeix-chat.conversation.v0"
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "empty messages",
			edit: func(archive *ConversationArchive) {
				archive.Messages = nil
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "invalid role",
			edit: func(archive *ConversationArchive) {
				archive.Messages[0].Role = "admin"
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "invalid content type",
			edit: func(archive *ConversationArchive) {
				archive.Messages[0].ContentType = "html"
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "invalid context policy json",
			edit: func(archive *ConversationArchive) {
				archive.Conversation.ContextPolicyJSON = "{"
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "missing parent reference",
			edit: func(archive *ConversationArchive) {
				archive.Messages[0].ParentPublicID = "missing_parent"
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "missing run reference",
			edit: func(archive *ConversationArchive) {
				archive.Messages[0].RunID = "missing_run"
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "malformed attachment",
			edit: func(archive *ConversationArchive) {
				archive.Messages[0].Attachments = []ConversationArchiveAttachment{{Kind: "url", FileName: "", FileSize: -1}}
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "sensitive trace payload",
			edit: func(archive *ConversationArchive) {
				archive.Messages[0].ProcessTrace = &ConversationArchiveProcessTrace{
					Process: &ConversationArchiveTraceBlock{PayloadJSON: `{"artifactID":123,"safe":"no"}`},
				}
			},
			want: ErrInvalidConversationArchive,
		},
		{
			name: "too many messages",
			edit: func(archive *ConversationArchive) {
				archive.Messages = make([]ConversationArchiveMessage, maxConversationArchiveMessages+1)
			},
			want: ErrConversationArchiveTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archive := base()
			tt.edit(archive)
			if err := validateConversationArchive(archive); !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

type archiveRepoFake struct {
	repository.ConversationRepository

	conversations map[string]*model.Conversation
	messages      []model.Message
	runs          []model.Run
	traces        []model.MessageTrace
	traceEvents   []model.MessageTraceEventRow

	createdConversation *model.Conversation
	createdMessages     []model.Message
	createdRuns         []model.Run
	createdAttachments  []model.Attachment
	upsertedTraces      []model.MessageTrace
	upsertedTraceEvents []model.MessageTraceEventRow
	incrementDelta      int
}

func (r *archiveRepoFake) WithConversationTransaction(ctx context.Context, fn func(repo repository.ConversationRepository) error) error {
	return fn(r)
}

func (r *archiveRepoFake) GetConversationByPublicID(ctx context.Context, publicID string, userID uint) (*model.Conversation, error) {
	item, ok := r.conversations[publicID]
	if !ok || item.UserID != userID {
		return nil, repository.ErrNotFound
	}
	copied := *item
	return &copied, nil
}

func (r *archiveRepoFake) ListMessages(ctx context.Context, conversationID uint, offset int, limit int) ([]model.Message, int64, error) {
	filtered := make([]model.Message, 0, len(r.messages))
	for _, message := range r.messages {
		if message.ConversationID == conversationID {
			filtered = append(filtered, message)
		}
	}
	total := int64(len(filtered))
	if offset >= len(filtered) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]model.Message(nil), filtered[offset:end]...), total, nil
}

func (r *archiveRepoFake) ListConversationMessageTracesByMessageIDs(ctx context.Context, messageIDs []uint) ([]model.MessageTrace, error) {
	allowed := uintSet(messageIDs)
	result := make([]model.MessageTrace, 0, len(r.traces))
	for _, trace := range r.traces {
		if allowed[trace.MessageID] {
			result = append(result, trace)
		}
	}
	return result, nil
}

func (r *archiveRepoFake) ListConversationMessageTraceEventsByMessageIDs(ctx context.Context, messageIDs []uint) ([]model.MessageTraceEventRow, error) {
	allowed := uintSet(messageIDs)
	result := make([]model.MessageTraceEventRow, 0, len(r.traceEvents))
	for _, event := range r.traceEvents {
		if allowed[event.MessageID] {
			result = append(result, event)
		}
	}
	return result, nil
}

func (r *archiveRepoFake) ListConversationRunsByRunIDs(ctx context.Context, userID uint, conversationID uint, runIDs []string) ([]model.Run, error) {
	allowed := stringSet(runIDs)
	result := make([]model.Run, 0, len(r.runs))
	for _, run := range r.runs {
		if run.UserID == userID && run.ConversationID == conversationID && allowed[run.RunID] {
			result = append(result, run)
		}
	}
	return result, nil
}

func (r *archiveRepoFake) CreateConversation(ctx context.Context, item *model.Conversation) error {
	copied := *item
	if copied.ID == 0 {
		copied.ID = 100 + uint(len(r.createdMessages)+len(r.createdRuns)+1)
	}
	*item = copied
	r.createdConversation = &copied
	return nil
}

func (r *archiveRepoFake) CreateConversationRun(ctx context.Context, item *model.Run) error {
	copied := *item
	if copied.ID == 0 {
		copied.ID = 300 + uint(len(r.createdRuns)+1)
	}
	*item = copied
	r.createdRuns = append(r.createdRuns, copied)
	return nil
}

func (r *archiveRepoFake) CreateMessage(ctx context.Context, item *model.Message) error {
	copied := *item
	if copied.ID == 0 {
		copied.ID = 200 + uint(len(r.createdMessages)+1)
	}
	*item = copied
	r.createdMessages = append(r.createdMessages, copied)
	return nil
}

func (r *archiveRepoFake) CreateAttachments(ctx context.Context, items []model.Attachment) error {
	r.createdAttachments = append(r.createdAttachments, items...)
	return nil
}

func (r *archiveRepoFake) UpsertConversationMessageTrace(ctx context.Context, item *model.MessageTrace) error {
	copied := *item
	r.upsertedTraces = append(r.upsertedTraces, copied)
	return nil
}

func (r *archiveRepoFake) UpsertConversationMessageTraceEvent(ctx context.Context, item *model.MessageTraceEventRow) error {
	copied := *item
	r.upsertedTraceEvents = append(r.upsertedTraceEvents, copied)
	return nil
}

func (r *archiveRepoFake) IncrementMessageCount(ctx context.Context, conversationID uint, delta int) error {
	r.incrementDelta += delta
	if r.createdConversation != nil && r.createdConversation.ID == conversationID {
		r.createdConversation.MessageCount += delta
	}
	return nil
}

func uintSet(values []uint) map[uint]bool {
	result := make(map[uint]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
