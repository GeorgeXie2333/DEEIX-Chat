package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIVideoGenerationCreatesPollsAndDownloads(t *testing.T) {
	var sawInputReference bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/videos":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST /v1/videos, got %s", r.Method)
			}
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				t.Fatalf("expected multipart request, got %q", r.Header.Get("Content-Type"))
			}
			if err := r.ParseMultipartForm(4 << 20); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			if got := r.FormValue("model"); got != "sora-2-pro" {
				t.Fatalf("unexpected model: %q", got)
			}
			if got := r.FormValue("prompt"); got != "make a short shot" {
				t.Fatalf("unexpected prompt: %q", got)
			}
			if got := r.FormValue("size"); got != "1920x1080" {
				t.Fatalf("unexpected size: %q", got)
			}
			if got := r.FormValue("seconds"); got != "12" {
				t.Fatalf("unexpected seconds: %q", got)
			}
			file, header, err := r.FormFile("input_reference")
			if err != nil {
				t.Fatalf("expected input_reference file: %v", err)
			}
			defer file.Close() //nolint:errcheck
			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("read input_reference: %v", err)
			}
			if header.Filename != "reference.png" || string(data) != "image-bytes" {
				t.Fatalf("unexpected input_reference: filename=%q data=%q", header.Filename, string(data))
			}
			sawInputReference = true
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "video_1",
				"status":  "queued",
				"size":    "1920x1080",
				"seconds": "12",
			})
		case "/v1/videos/video_1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      "video_1",
				"status":  "completed",
				"size":    "1920x1080",
				"seconds": "12",
			})
		case "/v1/videos/video_1/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(sampleMP4Bytes())
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := NewClient().Generate(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIVideoGenerations,
		BaseURL:       server.URL,
		APIKey:        "test-key",
		UpstreamModel: "sora-2-pro",
		ReadTimeoutMS: 5000,
	}, GenerateInput{
		Messages: []Message{{
			Role: "user",
			Parts: []ContentPart{
				{Kind: ContentPartText, Text: "make a short shot"},
				{Kind: ContentPartImage, MimeType: "image/png", FileName: "reference.png", Data: []byte("image-bytes")},
			},
		}},
		Options: map[string]interface{}{"size": "1920x1080", "seconds": "12"},
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if !sawInputReference {
		t.Fatal("expected input_reference to be sent")
	}
	if output.ResponseID != "video_1" || len(output.GeneratedVideos) != 1 {
		t.Fatalf("unexpected video output: %#v", output)
	}
	video := output.GeneratedVideos[0]
	if video.ID != "video_1" || video.MIMEType != "video/mp4" || string(video.Data) != string(sampleMP4Bytes()) {
		t.Fatalf("unexpected generated video: %#v", video)
	}
}

func TestOpenAIVideoGenerationStreamEmitsProgressStatus(t *testing.T) {
	var pollCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/videos":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST /v1/videos, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       "video_1",
				"status":   "queued",
				"progress": 0,
				"size":     "1280x720",
				"seconds":  "4",
			})
		case "/v1/videos/video_1":
			pollCount++
			if pollCount == 1 {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"id":       "video_1",
					"status":   "in_progress",
					"progress": 33,
					"size":     "1280x720",
					"seconds":  "4",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":       "video_1",
				"status":   "completed",
				"progress": 100,
				"size":     "1280x720",
				"seconds":  "4",
			})
		case "/v1/videos/video_1/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(sampleMP4Bytes())
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	var statuses []GeneratedVideoStatus
	output, err := NewClient().GenerateStream(context.Background(), RouteConfig{
		Protocol:      AdapterOpenAIVideoGenerations,
		BaseURL:       server.URL,
		APIKey:        "test-key",
		UpstreamModel: "sora-2",
		ReadTimeoutMS: 5000,
	}, GenerateInput{
		Messages: []Message{{Role: "user", Content: "make a short shot"}},
	}, func(event GenerateStreamEvent) error {
		if event.GeneratedVideoStatus != nil {
			statuses = append(statuses, *event.GeneratedVideoStatus)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateStream returned error: %v", err)
	}
	if pollCount != 2 {
		t.Fatalf("expected two status polls, got %d", pollCount)
	}
	if output.ResponseID != "video_1" || len(output.GeneratedVideos) != 1 {
		t.Fatalf("unexpected video output: %#v", output)
	}
	if len(statuses) != 3 {
		t.Fatalf("expected queued, in_progress, and completed statuses, got %#v", statuses)
	}
	assertVideoStatus(t, statuses[0], "queued", 0)
	assertVideoStatus(t, statuses[1], "in_progress", 33)
	assertVideoStatus(t, statuses[2], "completed", 100)
}

func TestOpenAIVideoGenerationLimitsSecondsToSupportedValues(t *testing.T) {
	body, _, _, err := buildOpenAIVideoGenerationMultipartRequest("sora-2", GenerateInput{
		Messages: []Message{{Role: "user", Content: "prompt"}},
		Options:  map[string]interface{}{"size": "1920x1080", "seconds": "20"},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	// The builder falls back to safe defaults when unsupported values reach the adapter.
	if !strings.Contains(string(body), "1280x720") || !strings.Contains(string(body), "\r\n4\r\n") {
		t.Fatalf("expected default size and seconds in multipart body: %s", string(body))
	}
}

func assertVideoStatus(t *testing.T, status GeneratedVideoStatus, wantStatus string, wantProgress int) {
	t.Helper()
	if status.ID != "video_1" || status.Status != wantStatus {
		t.Fatalf("unexpected status: %#v", status)
	}
	if status.Progress == nil || *status.Progress != wantProgress {
		t.Fatalf("expected progress %d, got %#v", wantProgress, status.Progress)
	}
}

func sampleMP4Bytes() []byte {
	return []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0x00, 0x00, 0x02, 0x00, 'i', 's', 'o', 'm', 'm', 'p', '4', '2'}
}
