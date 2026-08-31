package openapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestHTTPChatImageResolverAlwaysUsesStrictPublicHTTPSPolicy(t *testing.T) {
	resolver := NewHTTPChatImageResolver(1024)
	if _, err := resolver.ResolveChatImage(context.Background(), "https://127.0.0.1/image.png"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected loopback image URL to be rejected even without the global SSRF switch, got %v", err)
	}

	transport, ok := resolver.client.Transport.(*http.Transport)
	if !ok || transport.DialContext == nil {
		t.Fatal("expected the resolver to use a protected HTTP transport")
	}
	if _, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:443"); !errors.Is(err, security.ErrUnsafeOutboundURL) {
		t.Fatalf("expected protected dialer to reject a private address, got %v", err)
	}
}

func TestHTTPChatImageResolverValidatesEveryRedirect(t *testing.T) {
	resolver := NewHTTPChatImageResolver(1024)
	if resolver.client.CheckRedirect == nil {
		t.Fatal("expected redirect validation")
	}

	for _, rawURL := range []string{
		"http://example.com/image.png",
		"https://user@example.com/image.png",
		"https://127.0.0.1/image.png",
	} {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			t.Fatalf("build redirect request for %q: %v", rawURL, err)
		}
		if err := resolver.client.CheckRedirect(req, nil); err == nil {
			t.Fatalf("expected redirect target %q to be rejected", rawURL)
		}
	}

	publicReq, err := http.NewRequest(http.MethodGet, "https://cdn.example/image.png", nil)
	if err != nil {
		t.Fatalf("build public redirect request: %v", err)
	}
	if err := resolver.client.CheckRedirect(publicReq, nil); err != nil {
		t.Fatalf("expected public HTTPS redirect to remain supported, got %v", err)
	}
}

func TestBuildGenerateInputFromChatCompletionCapsImageCountAcrossMessages(t *testing.T) {
	messages := make([]interface{}, 0, defaultOpenAPIImageMaxCount+1)
	for index := 0; index < defaultOpenAPIImageMaxCount+1; index++ {
		messages = append(messages, userImageMessage(fmt.Sprintf("https://cdn.example/%d.png", index)))
	}

	calls := 0
	resolver := chatImageResolverFunc(func(_ context.Context, _ string) (llm.ContentPart, error) {
		calls++
		return llm.ContentPart{Kind: llm.ContentPartImage, MimeType: "image/png", Data: []byte("x")}, nil
	})
	_, err := buildGenerateInputFromChatCompletion(context.Background(), map[string]interface{}{"messages": messages}, resolver)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected excess images to be rejected, got %v", err)
	}
	if calls != defaultOpenAPIImageMaxCount {
		t.Fatalf("expected resolver to stop before image %d, got %d calls", defaultOpenAPIImageMaxCount+1, calls)
	}
}

func TestParseChatCompletionMessagesCapsAggregateRemoteImageBytes(t *testing.T) {
	resolver := &recordingLimitedImageResolver{data: []byte("abc")}
	messages := []interface{}{
		userImageMessage("https://cdn.example/one.png"),
		userImageMessage("https://cdn.example/two.png"),
	}
	_, err := parseChatCompletionMessagesWithImageBudget(context.Background(), messages, resolver, newChatImageBudget(2, 5))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected aggregate image bytes to be rejected, got %v", err)
	}
	if len(resolver.limits) != 2 || resolver.limits[0] != 5 || resolver.limits[1] != 2 {
		t.Fatalf("expected remaining aggregate limits [5 2], got %#v", resolver.limits)
	}
}

func TestParseChatCompletionMessagesAllowsImagesWithinAggregateBudget(t *testing.T) {
	resolver := &recordingLimitedImageResolver{data: []byte("abc")}
	messages := []interface{}{
		userImageMessage("https://cdn.example/one.png"),
		userImageMessage("https://cdn.example/two.png"),
	}
	parsed, err := parseChatCompletionMessagesWithImageBudget(context.Background(), messages, resolver, newChatImageBudget(2, 6))
	if err != nil {
		t.Fatalf("expected aggregate-safe images to be accepted, got %v", err)
	}
	if len(parsed) != 2 || len(parsed[0].Parts) != 1 || len(parsed[1].Parts) != 1 {
		t.Fatalf("unexpected parsed messages: %#v", parsed)
	}
	if len(resolver.limits) != 2 || resolver.limits[0] != 6 || resolver.limits[1] != 3 {
		t.Fatalf("expected remaining aggregate limits [6 3], got %#v", resolver.limits)
	}
}

func userImageMessage(rawURL string) map[string]interface{} {
	return map[string]interface{}{
		"role": "user",
		"content": []interface{}{
			map[string]interface{}{"type": "image_url", "image_url": rawURL},
		},
	}
}

type recordingLimitedImageResolver struct {
	data   []byte
	limits []int64
}

func (r *recordingLimitedImageResolver) ResolveChatImage(_ context.Context, _ string) (llm.ContentPart, error) {
	return llm.ContentPart{Kind: llm.ContentPartImage, MimeType: "image/png", Data: append([]byte(nil), r.data...)}, nil
}

func (r *recordingLimitedImageResolver) resolveChatImageWithLimit(_ context.Context, _ string, maxBytes int64) (llm.ContentPart, error) {
	r.limits = append(r.limits, maxBytes)
	return r.ResolveChatImage(context.Background(), "")
}
