package settings

import (
	"context"
	"testing"

	domainsettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/settings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

type settingsSeedRepo struct {
	items map[string]domainsettings.SystemSetting
}

func newSettingsSeedRepo(items ...domainsettings.SystemSetting) *settingsSeedRepo {
	repo := &settingsSeedRepo{items: map[string]domainsettings.SystemSetting{}}
	for _, item := range items {
		repo.items[item.Namespace+":"+item.Key] = item
	}
	return repo
}

func (r *settingsSeedRepo) ListAll(context.Context) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0, len(r.items))
	for _, item := range r.items {
		items = append(items, item)
	}
	return items, nil
}

func (r *settingsSeedRepo) ListByNamespace(_ context.Context, namespace string) ([]domainsettings.SystemSetting, error) {
	items := make([]domainsettings.SystemSetting, 0)
	for _, item := range r.items {
		if item.Namespace == namespace {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *settingsSeedRepo) Upsert(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, item := range items {
		r.items[item.Namespace+":"+item.Key] = item
	}
	return nil
}

func (r *settingsSeedRepo) UpsertWithDescription(_ context.Context, items []domainsettings.SystemSetting) error {
	for _, item := range items {
		key := item.Namespace + ":" + item.Key
		if _, ok := r.items[key]; !ok {
			r.items[key] = item
		}
	}
	return nil
}

func (r *settingsSeedRepo) Delete(_ context.Context, namespace string, key string) error {
	delete(r.items, namespace+":"+key)
	return nil
}

func TestSeedMigratesLegacyDefaultAllowedMIMETypes(t *testing.T) {
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "allowed_mime_types",
		Value:     legacyDefaultAllowedMIMETypes,
		ValueType: "string",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["file:allowed_mime_types"].Value
	if got != defaultAllowedMIMETypes {
		t.Fatalf("expected legacy MIME defaults to migrate, got %q", got)
	}
}

func TestSeedKeepsCustomAllowedMIMETypes(t *testing.T) {
	custom := "image/png,text/plain"
	repo := newSettingsSeedRepo(domainsettings.SystemSetting{
		Namespace: "file",
		Key:       "allowed_mime_types",
		Value:     custom,
		ValueType: "string",
	})
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	got := repo.items["file:allowed_mime_types"].Value
	if got != custom {
		t.Fatalf("expected custom MIME defaults to stay unchanged, got %q", got)
	}
}

func TestSeedAddsDefaultStripeFeeRate(t *testing.T) {
	repo := newSettingsSeedRepo()
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	item, ok := repo.items["billing:stripe_fee_rate_percent"]
	if !ok {
		t.Fatal("expected stripe fee rate setting to be seeded")
	}
	if item.Value != "0" {
		t.Fatalf("stripe fee rate default = %q, want 0", item.Value)
	}
}

func TestSeedAddsDefaultProviderMinimumTopUpAmounts(t *testing.T) {
	repo := newSettingsSeedRepo()
	service := NewService(repo, "")

	if err := service.Seed(context.Background(), config.Config{}); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	for _, key := range []string{
		"billing:stripe_minimum_top_up_amount_usd",
		"billing:epay_minimum_top_up_amount_usd",
	} {
		item, ok := repo.items[key]
		if !ok {
			t.Fatalf("expected %s setting to be seeded", key)
		}
		if item.Value != "0" {
			t.Fatalf("%s default = %q, want 0", key, item.Value)
		}
	}
}
