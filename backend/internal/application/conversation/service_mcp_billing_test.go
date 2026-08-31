package conversation

import (
	"context"
	"errors"
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

type mcpBillingToolResolverStub struct {
	tools   []domainmcp.Tool
	servers map[uint]*domainmcp.Server
}

func (s mcpBillingToolResolverStub) ListToolsByIDs(_ context.Context, ids []uint) ([]domainmcp.Tool, error) {
	wanted := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	result := make([]domainmcp.Tool, 0, len(ids))
	for _, tool := range s.tools {
		if _, ok := wanted[tool.ID]; ok {
			result = append(result, tool)
		}
	}
	return result, nil
}

func (s mcpBillingToolResolverStub) ListServers(context.Context) ([]domainmcp.Server, error) {
	return nil, nil
}

func (s mcpBillingToolResolverStub) GetServer(_ context.Context, id uint) (*domainmcp.Server, error) {
	return s.servers[id], nil
}

type mcpBillingRepositoryStub struct {
	repository.BillingRepository

	mode               string
	prepaidNanousd     int64
	pricing            *domainbilling.ModelPricing
	reservationRequest *domainbilling.UsageBalanceReservationRequest
	reservationErr     error
}

func (s *mcpBillingRepositoryStub) GetBillingMode(context.Context) (string, error) {
	return s.mode, nil
}

func (s *mcpBillingRepositoryStub) GetBillingPrepaidAmountNanousd(context.Context) (int64, error) {
	return s.prepaidNanousd, nil
}

func (s *mcpBillingRepositoryStub) GetModelPricing(context.Context, string) (*domainbilling.ModelPricing, error) {
	return s.pricing, nil
}

func (s *mcpBillingRepositoryStub) ReserveUsageBalance(_ context.Context, input domainbilling.UsageBalanceReservationRequest) (*domainbilling.UsageBalanceReservation, error) {
	s.reservationRequest = &input
	if s.reservationErr != nil {
		return nil, s.reservationErr
	}
	return &domainbilling.UsageBalanceReservation{
		UserID: input.UserID,
		RefNo:  input.RefNo,
		Mode:   input.Mode,
	}, nil
}

func newMCPBillingTestService(t *testing.T, prices ...int64) *Service {
	t.Helper()
	const dataEncryptionKey = "mcp-billing-test-key"
	token, err := secretbox.EncryptString(dataEncryptionKey, "mcp-token")
	if err != nil {
		t.Fatalf("encrypt MCP token: %v", err)
	}
	tools := make([]domainmcp.Tool, 0, len(prices))
	for index, price := range prices {
		tools = append(tools, domainmcp.Tool{
			ID:           uint(index + 1),
			ServerID:     10,
			Name:         "tool",
			Status:       "active",
			PriceNanousd: price,
		})
	}
	return &Service{
		cfg: config.NewRuntime(config.Config{
			MCPEnable:             true,
			DataEncryptionKey:     dataEncryptionKey,
			MCPMaxLLMCallsPerRun:  32,
			MCPMaxToolCallsPerRun: 3,
		}),
		mcpRepo: mcpBillingToolResolverStub{
			tools: tools,
			servers: map[uint]*domainmcp.Server{
				10: {
					ID:           10,
					Name:         "priced-mcp",
					BaseURL:      "https://mcp.example.test",
					AuthTokenEnc: token,
					Status:       "active",
				},
			},
		},
	}
}

func TestMinimumMCPReservationUsesHighestExecutableToolPriceAndToolCallCap(t *testing.T) {
	service := newMCPBillingTestService(t, 5, 11)

	amount, err := service.minimumMCPReservationNanousd(t.Context(), []uint{1, 2})
	if err != nil {
		t.Fatalf("minimumMCPReservationNanousd() error = %v", err)
	}
	if want := int64(33); amount != want {
		t.Fatalf("reservation = %d, want highest tool price * MCPMaxToolCallsPerRun = %d", amount, want)
	}
}

func TestMinimumMCPReservationSharesToolCallCapBetweenAttachmentAndFollowUpTools(t *testing.T) {
	service := newMCPBillingTestService(t, 11, 7)
	resolver := service.mcpRepo.(mcpBillingToolResolverStub)
	resolver.tools[0].AttachmentInputMode = domainmcp.AttachmentInputModeImage
	resolver.tools[0].AttachmentArgument = "image"
	service.mcpRepo = resolver

	amount, err := service.minimumMCPReservationNanousd(t.Context(), []uint{1, 2})
	if err != nil {
		t.Fatalf("minimumMCPReservationNanousd() error = %v", err)
	}
	// Image processors reduce remainingToolCalls before follow-up MCP calls run;
	// both paths therefore share the same three-call run budget.
	if want := int64(33); amount != want {
		t.Fatalf("reservation = %d, want shared tool-call cap of %d", amount, want)
	}
}

func TestAuthorizeSendMessageUsageReservesForFreeModelWithPricedMCP(t *testing.T) {
	service := newMCPBillingTestService(t, 11)
	billingRepo := &mcpBillingRepositoryStub{
		mode:           "usage",
		prepaidNanousd: 4,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "free-model",
			Currency:          "USD",
			IsFree:            true,
		},
	}
	service.billingSvc = appbilling.NewService(billingRepo)

	authorization, err := service.AuthorizeSendMessageUsage(t.Context(), SendMessageBillingInput{
		UserID:            1,
		PlatformModelName: "free-model",
		ClientRunID:       "run_free_priced_mcp",
		SelectedToolIDs:   []uint{1},
	})
	if err != nil {
		t.Fatalf("AuthorizeSendMessageUsage() error = %v", err)
	}
	if authorization == nil || authorization.Reservation == nil {
		t.Fatalf("authorization = %#v, want reservation", authorization)
	}
	if authorization.MCPToolPriceNanousdByID[1] != 11 {
		t.Fatalf("authorization MCP price snapshot = %#v, want tool 1 at 11", authorization.MCPToolPriceNanousdByID)
	}
	if billingRepo.reservationRequest == nil || billingRepo.reservationRequest.RequestedNanousd != 33 {
		t.Fatalf("reservation request = %#v, want 33", billingRepo.reservationRequest)
	}
}

func TestAuthorizedMCPPriceSnapshotFreezesPriceAndExecutableSelection(t *testing.T) {
	service := newMCPBillingTestService(t, 11)
	billingRepo := &mcpBillingRepositoryStub{
		mode:           "usage",
		prepaidNanousd: 100,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "free-model",
			Currency:          "USD",
			IsFree:            true,
		},
	}
	service.billingSvc = appbilling.NewService(billingRepo)
	authorization, err := service.AuthorizeSendMessageUsage(t.Context(), SendMessageBillingInput{
		UserID:            1,
		PlatformModelName: "free-model",
		ClientRunID:       "run_price_snapshot",
		SelectedToolIDs:   []uint{1},
	})
	if err != nil {
		t.Fatalf("AuthorizeSendMessageUsage() error = %v", err)
	}

	resolver := service.mcpRepo.(mcpBillingToolResolverStub)
	resolver.tools[0].PriceNanousd = 99
	service.mcpRepo = resolver
	runtime, err := service.resolveSelectedToolRuntime(t.Context(), []uint{1})
	if err != nil {
		t.Fatalf("resolveSelectedToolRuntime() error = %v", err)
	}
	runtime = runtime.withAuthorizedMCPPrices(authorization.MCPToolPriceNanousdByID)
	if len(runtime.mcpBindings) != 1 {
		t.Fatalf("authorized runtime bindings = %#v, want one tool", runtime.mcpBindings)
	}
	for _, binding := range runtime.mcpBindings {
		if binding.PriceNanousd != 11 {
			t.Fatalf("runtime used changed MCP price %d, want authorized price 11", binding.PriceNanousd)
		}
	}

	runtime = runtime.withAuthorizedMCPPrices(map[uint]int64{})
	if len(runtime.definitions) != 0 || len(runtime.mcpBindings) != 0 || runtime.attachmentProcessor != nil {
		t.Fatalf("empty authorization snapshot must expose no newly enabled tools: %#v", runtime)
	}
}

func TestAuthorizeSendMessageUsageRejectsInsufficientBalanceBeforePricedMCPCanRun(t *testing.T) {
	service := newMCPBillingTestService(t, 11)
	billingRepo := &mcpBillingRepositoryStub{
		mode:           "usage",
		reservationErr: repository.ErrInsufficientBalance,
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "free-model",
			Currency:          "USD",
			IsFree:            true,
		},
	}
	service.billingSvc = appbilling.NewService(billingRepo)

	_, err := service.AuthorizeSendMessageUsage(t.Context(), SendMessageBillingInput{
		UserID:            1,
		PlatformModelName: "free-model",
		ClientRunID:       "run_free_priced_mcp_without_balance",
		SelectedToolIDs:   []uint{1},
	})
	if !errors.Is(err, appbilling.ErrUsageBalanceInsufficient) {
		t.Fatalf("AuthorizeSendMessageUsage() error = %v, want insufficient-balance rejection before execution", err)
	}
}

func TestAuthorizeSendMessageUsageKeepsFreeModelWithoutReservationForUnpricedMCP(t *testing.T) {
	service := newMCPBillingTestService(t, 0)
	billingRepo := &mcpBillingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "free-model",
			Currency:          "USD",
			IsFree:            true,
		},
	}
	service.billingSvc = appbilling.NewService(billingRepo)

	authorization, err := service.AuthorizeSendMessageUsage(t.Context(), SendMessageBillingInput{
		UserID:            1,
		PlatformModelName: "free-model",
		ClientRunID:       "run_free_unpriced_mcp",
		SelectedToolIDs:   []uint{1},
	})
	if err != nil {
		t.Fatalf("AuthorizeSendMessageUsage() error = %v", err)
	}
	if authorization == nil || authorization.Reservation != nil || billingRepo.reservationRequest != nil {
		t.Fatalf("unpriced MCP must preserve free-model fast path: authorization=%#v request=%#v", authorization, billingRepo.reservationRequest)
	}
}
