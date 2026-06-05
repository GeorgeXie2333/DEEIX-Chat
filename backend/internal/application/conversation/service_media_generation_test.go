package conversation

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

func TestNormalizeMediaImageEditInputConvertsCMYKJPEGToPNG(t *testing.T) {
	src := image.NewCMYK(image.Rect(0, 0, 2, 2))
	src.SetCMYK(0, 0, color.CMYK{C: 255, M: 0, Y: 0, K: 0})
	src.SetCMYK(1, 0, color.CMYK{C: 0, M: 255, Y: 0, K: 0})
	src.SetCMYK(0, 1, color.CMYK{C: 0, M: 0, Y: 255, K: 0})
	src.SetCMYK(1, 1, color.CMYK{C: 0, M: 0, Y: 0, K: 32})

	var input bytes.Buffer
	if err := jpeg.Encode(&input, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode source jpeg: %v", err)
	}

	got, mimeType, err := normalizeMediaImageEditInput(input.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatalf("normalize image edit input: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("expected normalized PNG MIME, got %q", mimeType)
	}
	if !bytes.HasPrefix(got, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		prefixLen := len(got)
		if prefixLen > 8 {
			prefixLen = 8
		}
		t.Fatalf("expected normalized PNG bytes, got prefix % x", got[:prefixLen])
	}
	decoded, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode normalized PNG: %v", err)
	}
	if decoded.Bounds().Dx() != 2 || decoded.Bounds().Dy() != 2 {
		t.Fatalf("expected dimensions to be preserved, got %v", decoded.Bounds())
	}
}

func TestMediaImageEditInputFileNameMatchesNormalizedMIME(t *testing.T) {
	got := mediaImageEditInputFileName("IMG_4442.jpeg", "image/png")
	if got != "IMG_4442.png" {
		t.Fatalf("expected normalized filename extension, got %q", got)
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

func TestMediaImageStreamEnabledUsesModelCapabilitiesOverride(t *testing.T) {
	tests := []struct {
		name             string
		protocol         string
		upstreamModel    string
		capabilitiesJSON string
		want             bool
	}{
		{
			name:          "openai gpt image defaults to stream",
			protocol:      llm.AdapterOpenAIImageGenerations,
			upstreamModel: "gpt-image-2",
			want:          true,
		},
		{
			name:             "explicit false disables stream",
			protocol:         llm.AdapterOpenAIImageGenerations,
			upstreamModel:    "gpt-image-2",
			capabilitiesJSON: `{"image":{"stream":false}}`,
			want:             false,
		},
		{
			name:             "explicit true preserves protocol default",
			protocol:         llm.AdapterOpenAIImageGenerations,
			upstreamModel:    "gpt-image-2",
			capabilitiesJSON: `{"image":{"stream":true}}`,
			want:             true,
		},
		{
			name:             "invalid json keeps protocol default",
			protocol:         llm.AdapterGoogleImageGeneration,
			upstreamModel:    "gemini-3-pro-image",
			capabilitiesJSON: `{`,
			want:             true,
		},
		{
			name:             "unsupported protocol cannot be enabled by capabilities",
			protocol:         llm.AdapterXAIImage,
			upstreamModel:    "grok-2-image",
			capabilitiesJSON: `{"image":{"stream":true}}`,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mediaImageStreamEnabled(tt.protocol, tt.upstreamModel, tt.capabilitiesJSON)
			if got != tt.want {
				t.Fatalf("mediaImageStreamEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
