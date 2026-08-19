package conversation

import (
	"bytes"
	"context"
	"strings"
	"time"

	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

type assistantGeneratedImageSaveInput struct {
	UserID         uint
	ConversationID uint
	MessageID      uint
	ModelName      string
	Images         []llm.GeneratedImage
}

func (s *Service) saveAssistantGeneratedImages(ctx context.Context, input assistantGeneratedImageSaveInput) ([]model.FileObject, []model.Attachment, error) {
	if len(input.Images) == 0 {
		return nil, nil, nil
	}
	uploaded := make([]model.FileObject, 0, len(input.Images))
	attachmentRows := make([]model.Attachment, 0, len(input.Images))
	now := time.Now()
	for i, image := range input.Images {
		data, mimeType, err := s.readGeneratedImage(ctx, image, "")
		if err != nil {
			return nil, nil, err
		}
		fileName := generatedImageFileName(input.ModelName, now, i, len(input.Images), mimeType)
		uploadResult, err := s.UploadFile(ctx, appupload.UploadFileInput{
			UserID:       input.UserID,
			Purpose:      "generated_image",
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
	return uploaded, attachmentRows, nil
}

func appendGeneratedImageMarkdown(text string, files []model.FileObject) string {
	markdown := strings.TrimSpace(generatedImageMarkdown(files))
	if markdown == "" {
		return text
	}
	if strings.TrimSpace(text) == "" {
		return markdown
	}
	return strings.TrimRight(text, "\n") + "\n\n" + markdown
}
