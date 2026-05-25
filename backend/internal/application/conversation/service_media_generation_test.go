package conversation

import (
	"bytes"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
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
