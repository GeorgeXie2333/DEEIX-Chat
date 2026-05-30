package conversation

import (
	"bytes"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestDetectGeneratedImageMIMERejectsNonImageBytes(t *testing.T) {
	_, _, err := validateGeneratedImageBytes([]byte("<html>not an image</html>"), "image/png")
	if err == nil {
		t.Fatal("expected non-image generated output to be rejected")
	}
}

func TestDetectGeneratedImageMIMEUsesActualImageBytes(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}
	got, mimeType, err := validateGeneratedImageBytes(data, "image/png")
	if err != nil {
		t.Fatalf("expected jpeg bytes to pass validation: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("expected validation to return original bytes")
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("expected actual jpeg MIME, got %q", mimeType)
	}
}

func TestStripBase64DataURLPrefix(t *testing.T) {
	got := stripBase64DataURLPrefix("data:image/png;base64, aGVsbG8= ")
	if got != "aGVsbG8=" {
		t.Fatalf("unexpected stripped data URL: %q", got)
	}
}

func TestAppendGeneratedImageMarkdownAllowsImageOnlyAssistantMessage(t *testing.T) {
	files := []model.FileObject{{FileID: "file_generated"}}
	got := appendGeneratedImageMarkdown("", files)
	if got != "![Generated image](/api/v1/files/file_generated/content)" {
		t.Fatalf("unexpected image-only assistant content: %q", got)
	}
}

func TestAppendGeneratedImageMarkdownPreservesAssistantText(t *testing.T) {
	files := []model.FileObject{{FileID: "file_generated"}}
	got := appendGeneratedImageMarkdown("Here is the poster.", files)
	want := "Here is the poster.\n\n![Generated image](/api/v1/files/file_generated/content)"
	if got != want {
		t.Fatalf("unexpected assistant content with image: %q", got)
	}
}

func TestSanitizeOpenAIVideoGenerationOptions(t *testing.T) {
	got := sanitizeOpenAIVideoGenerationOptions("sora-2", map[string]interface{}{
		"size":    "1920x1080",
		"seconds": "20",
	})
	if len(got) != 0 {
		t.Fatalf("expected unsupported Sora 2 options to be removed, got %#v", got)
	}

	got = sanitizeOpenAIVideoGenerationOptions("sora-2-pro", map[string]interface{}{
		"size":    "1920x1080",
		"seconds": "12",
	})
	if got["size"] != "1920x1080" || got["seconds"] != "12" {
		t.Fatalf("expected supported Sora 2 Pro options to pass, got %#v", got)
	}
}

func TestEmitMediaVideoStatusIncludesProgress(t *testing.T) {
	progress := 33
	var gotType string
	var gotPayload map[string]interface{}

	err := emitMediaVideoStatus(func(eventType string, payload map[string]interface{}) error {
		gotType = eventType
		gotPayload = payload
		return nil
	}, &llm.GeneratedVideoStatus{
		ID:       "video_1",
		Status:   "in_progress",
		Progress: &progress,
		Size:     "1280x720",
		Seconds:  "4",
	})
	if err != nil {
		t.Fatalf("emitMediaVideoStatus returned error: %v", err)
	}
	if gotType != "media_status" {
		t.Fatalf("unexpected event type: %q", gotType)
	}
	if gotPayload["status"] != "running" || gotPayload["message"] != "generating video" {
		t.Fatalf("unexpected media status payload: %#v", gotPayload)
	}
	if gotPayload["upstream_status"] != "in_progress" || gotPayload["video_id"] != "video_1" {
		t.Fatalf("expected upstream video metadata, got %#v", gotPayload)
	}
	if gotPayload["progress"] != 33 {
		t.Fatalf("expected progress 33, got %#v", gotPayload["progress"])
	}
}

func TestValidateGeneratedVideoBytesRequiresMP4(t *testing.T) {
	if _, _, err := validateGeneratedVideoBytes([]byte("not a video"), "video/mp4"); err == nil {
		t.Fatal("expected invalid video bytes to be rejected")
	}
	data := []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
	got, mimeType, err := validateGeneratedVideoBytes(data, "application/octet-stream")
	if err != nil {
		t.Fatalf("expected mp4 bytes to pass: %v", err)
	}
	if !bytes.Equal(got, data) || mimeType != "video/mp4" {
		t.Fatalf("unexpected validated video: mime=%q data=%v", mimeType, got)
	}
}
