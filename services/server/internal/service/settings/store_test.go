package settings

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation"
	"github.com/mediago-dev/mediago-drama/packages/core/pkg/generation/libtv"
)

type memoryAPIKeyStore struct {
	mu     sync.RWMutex
	values map[string]string
}

func (store *memoryAPIKeyStore) Get(keyName string) (string, string, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	value := store.values[keyName]
	if value == "" {
		return "", "none", nil
	}
	return value, "settings", nil
}

func (store *memoryAPIKeyStore) Set(keyName string, value string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.values[keyName] = value
	return nil
}

func (store *memoryAPIKeyStore) Clear(keyName string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.values, keyName)
	return nil
}

func TestSettingsSetListAndClearAPIKey(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{}})
	settings.SetModelPlatforms([]string{ModelPlatformOpenRouter})

	list, err := settings.SetAPIKey(context.Background(), "openrouter", "sk-service")
	if err != nil {
		t.Fatalf("SetAPIKey returned error: %v", err)
	}
	provider := providerByID(t, list, "openrouter")
	if !provider.Configured || provider.Source != "settings" || provider.Masked == "" {
		t.Fatalf("provider after set = %#v, want configured settings provider", provider)
	}
	if !stringSliceContains(provider.Capabilities, "generation") || !stringSliceContains(provider.Capabilities, "agent") {
		t.Fatalf("openrouter capabilities = %#v, want generation and agent", provider.Capabilities)
	}

	list, err = settings.ClearAPIKey(context.Background(), "openrouter")
	if err != nil {
		t.Fatalf("ClearAPIKey returned error: %v", err)
	}
	provider = providerByID(t, list, "openrouter")
	if provider.Configured || provider.Source != "none" || provider.Masked != "" {
		t.Fatalf("provider after clear = %#v, want unconfigured provider", provider)
	}
}

func TestSettingsListAPIKeysIncludesGenerationAndAgentProviders(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{}})
	settings.SetModelPlatforms([]string{
		ModelPlatformMediago,
		ModelPlatformOpenRouter,
		ModelPlatformDMXAPI,
	})

	list, err := settings.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys returned error: %v", err)
	}

	provider := providerByID(t, list, "deepseek")
	if provider.Configured || provider.Source != "none" {
		t.Fatalf("deepseek provider = %#v, want unconfigured provider", provider)
	}
	if provider.Label != "DeepSeek" ||
		!stringSliceContains(provider.Capabilities, "agent") ||
		!stringSliceContains(provider.Capabilities, "generation") {
		t.Fatalf("deepseek provider = %#v, want generation and agent DeepSeek provider", provider)
	}

	provider = providerByID(t, list, "dmx")
	if provider.Configured || provider.Source != "none" {
		t.Fatalf("dmx provider = %#v, want unconfigured provider", provider)
	}
	if provider.Label != "DMX" ||
		!stringSliceContains(provider.Capabilities, "agent") ||
		!stringSliceContains(provider.Capabilities, "generation") {
		t.Fatalf("dmx provider = %#v, want generation and agent DMX provider", provider)
	}

	provider = providerByID(t, list, "mediago")
	if provider.Label != "MediaGo" ||
		!stringSliceContains(provider.Capabilities, "agent") ||
		!stringSliceContains(provider.Capabilities, "generation") {
		t.Fatalf("mediago provider = %#v, want generation and agent MediaGo provider", provider)
	}
}

func TestSettingsListAPIKeysKeepsConfigurationEntriesVisibleOutsideRuntimeAllowlist(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{
		generation.ProviderMediago:    "mgak-existing",
		generation.ProviderOpenRouter: "sk-openrouter-existing",
		generation.ProviderDMX:        "sk-dmx-existing",
	}})
	settings.SetModelPlatforms([]string{ModelPlatformOpenRouter})
	settings.SetGenerationCLIs([]string{})

	list, err := settings.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys returned error: %v", err)
	}
	for _, providerID := range []string{
		generation.ProviderMediago,
		generation.ProviderOpenRouter,
		generation.ProviderDMX,
		generation.ProviderSpeechAPI,
		generation.ProviderVideoAPI,
		generation.ProviderJimeng,
		generation.ProviderLibTV,
		generation.ProviderXiaoyunque,
		agentModelProviderAIHubMix,
	} {
		if !apiKeyProviderExists(list, providerID) {
			t.Fatalf("providers = %#v, want configuration entry %q visible", list.Providers, providerID)
		}
	}
	if settings.GenerationProviderEnabled(generation.ProviderMediago) {
		t.Fatal("MediaGo runtime provider should remain disabled outside MODEL_PLATFORM")
	}
	if settings.GenerationProviderEnabled(generation.ProviderDMX) {
		t.Fatal("DMX runtime provider should remain disabled outside MODEL_PLATFORM")
	}
	if !settings.GenerationProviderEnabled(generation.ProviderOpenRouter) {
		t.Fatal("OpenRouter runtime provider should remain enabled by MODEL_PLATFORM")
	}
}

func TestSettingsListModelPlatformsDefaultsAndOverrides(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{}})

	list := settings.ListModelPlatforms(context.Background())
	if len(list.Platforms) != 0 {
		t.Fatalf("default platforms = %#v, want explicit configuration before exposing routes", list.Platforms)
	}

	settings.SetGenerationCLIs([]string{})
	settings.SetModelPlatforms([]string{ModelPlatformOpenRouter, ModelPlatformDMXAPI})
	list = settings.ListModelPlatforms(context.Background())
	if len(list.Platforms) != 2 ||
		list.Platforms[0].ID != ModelPlatformOpenRouter ||
		list.Platforms[1].APIKeyProviderID != "dmx" {
		t.Fatalf("override platforms = %#v, want openrouter then dmxapi", list.Platforms)
	}
}

func TestSettingsGenerationCLIProvidersFollowConfiguration(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{}})
	settings.SetGenerationCLIs([]string{"libtv", "pippit"})

	list := settings.ListModelPlatforms(context.Background())
	if !modelPlatformExists(list, generation.ProviderLibTV) ||
		!modelPlatformExists(list, generation.ProviderXiaoyunque) ||
		modelPlatformExists(list, generation.ProviderJimeng) {
		t.Fatalf("platforms = %#v, want libtv and xiaoyunque CLI platforms only", list.Platforms)
	}

	keys, err := settings.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys returned error: %v", err)
	}
	if !apiKeyProviderExists(keys, generation.ProviderLibTV) ||
		!apiKeyProviderExists(keys, generation.ProviderXiaoyunque) ||
		!apiKeyProviderExists(keys, generation.ProviderJimeng) {
		t.Fatalf("providers = %#v, want credentials manageable independently of enabled routes", keys.Providers)
	}

	if _, err := settings.SetAPIKey(context.Background(), generation.ProviderXiaoyunque, "xyq-key"); err != nil {
		t.Fatalf("SetAPIKey xiaoyunque returned error: %v", err)
	}
	if _, err := settings.SetAPIKey(context.Background(), generation.ProviderJimeng, "oauth:old"); err != nil {
		t.Fatalf("saving inactive CLI credential: %v", err)
	}
	if modelPlatformExists(settings.ListModelPlatforms(context.Background()), generation.ProviderJimeng) {
		t.Fatal("saving a credential must not implicitly enable its CLI route")
	}
}

func TestSettingsLibTVProjectBindingsPersist(t *testing.T) {
	ctx := context.Background()
	settings := NewSettingsWithStores(
		&memoryAPIKeyStore{values: map[string]string{}},
		nil,
		&memoryAppSettingStore{values: map[string]string{}},
	)

	if _, ok, err := settings.GetLibTVProjectBinding(ctx, "project-alpha"); err != nil || ok {
		t.Fatalf("empty GetLibTVProjectBinding ok=%v err=%v, want missing without error", ok, err)
	}
	if err := settings.SaveLibTVProjectBinding(ctx, libtv.ProjectBinding{
		InternalProjectID:   "project-alpha",
		InternalProjectName: "天机阁",
		ProjectID:           "22222222-3333-4444-5555-666666666666",
		ProjectName:         "MediaGo - 天机阁",
		CreatedAt:           "2026-07-11T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveLibTVProjectBinding returned error: %v", err)
	}

	binding, ok, err := settings.GetLibTVProjectBinding(ctx, "project-alpha")
	if err != nil || !ok {
		t.Fatalf("GetLibTVProjectBinding ok=%v err=%v, want stored binding", ok, err)
	}
	if binding.ProjectID != "22222222-3333-4444-5555-666666666666" ||
		binding.InternalProjectName != "天机阁" ||
		binding.ProjectName != "MediaGo - 天机阁" ||
		binding.CreatedAt != "2026-07-11T00:00:00Z" ||
		binding.UpdatedAt == "" {
		t.Fatalf("binding = %#v, want persisted LibTV project binding", binding)
	}
}

func TestParseGenerationCLIProviderIDs(t *testing.T) {
	got, err := ParseGenerationCLIProviderIDs("dreamina,libtv,pippit,xiaoyunque,pippit-tool-cli")
	if err != nil {
		t.Fatalf("ParseGenerationCLIProviderIDs returned error: %v", err)
	}
	if strings.Join(got, ",") != "jimeng,libtv,xiaoyunque" {
		t.Fatalf("ids = %#v, want deduped provider ids", got)
	}

	got, err = ParseGenerationCLIProviderIDs("none")
	if err != nil {
		t.Fatalf("ParseGenerationCLIProviderIDs(none) returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ids = %#v, want empty list for none", got)
	}
	if _, err := ParseGenerationCLIProviderIDs("auto"); err == nil {
		t.Fatal("ParseGenerationCLIProviderIDs(auto) returned nil error, want unsupported CLI error")
	}
}

func TestSettingsListModelPlatformsUsesMediagoGatewayModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/models/user" {
			t.Fatalf("request path = %q, want /api/v1/models/user", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer mgak-test" {
			t.Fatalf("authorization = %q, want bearer key", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{
				"data": [
					{"id":"glm-5.2","name":"GLM-5.2","kind":"text"},
					{"id":"qwen-mt-plus","name":"Qwen MT Plus","kind":"text"},
					{"id":"doubao-seedream-5-0-lite","name":"Seedream 5.0 Lite","kind":"image"},
					{"id":"gemini-disabled","name":"Gemini Disabled","kind":"text","enabled":false},
					{"id":"hidden-model","name":"Hidden Model","kind":"text","status":"hidden"}
				]
			}`)
	}))
	defer server.Close()

	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{
		ModelPlatformMediago: "mgak-test",
	}})
	settings.SetGenerationCLIs([]string{})
	settings.SetMediagoBaseURL(server.URL + "/api/v1")
	settings.SetModelPlatforms([]string{ModelPlatformMediago})

	list := settings.ListModelPlatforms(context.Background())
	if len(list.Platforms) != 1 {
		t.Fatalf("platforms = %#v, want one mediago platform", list.Platforms)
	}
	groups := map[string][]string{}
	for _, group := range list.Platforms[0].ModelGroups {
		groups[group.Label] = group.Models
	}
	if !stringSliceContainsFold(groups["智谱 GLM"], "GLM-5.2") ||
		!stringSliceContainsFold(groups["通义千问"], "Qwen MT Plus") ||
		!stringSliceContainsFold(groups["字节"], "Seedream 5.0 Lite") {
		t.Fatalf("model groups = %#v, want gateway vendor groups", list.Platforms[0].ModelGroups)
	}
	if stringSliceContainsFold(groups["Google Gemini"], "Gemini Disabled") ||
		stringSliceContainsFold(groups["其他"], "Hidden Model") {
		t.Fatalf("model groups = %#v, want disabled gateway models hidden", list.Platforms[0].ModelGroups)
	}
}

func TestSettingsSetAPIKeyValidation(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{}})

	if _, err := settings.SetAPIKey(context.Background(), "missing", "sk"); err != ErrAPIKeyProviderNotFound {
		t.Fatalf("SetAPIKey missing provider error = %v, want ErrAPIKeyProviderNotFound", err)
	}
	if _, err := settings.SetAPIKey(context.Background(), "openrouter", " "); err != ErrAPIKeyRequired {
		t.Fatalf("SetAPIKey blank key error = %v, want ErrAPIKeyRequired", err)
	}
}

func TestSettingsClearJimengAPIKeyRunsLogout(t *testing.T) {
	store := &memoryAPIKeyStore{values: map[string]string{"jimeng": "oauth:old"}}
	settings := NewSettings(store)
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.log")
	binPath := writeSettingsCLIFixture(t, "logout", argsPath)
	settings.SetJimengCLIPaths(binPath, "")

	list, err := settings.ClearAPIKey(context.Background(), "jimeng")
	if err != nil {
		t.Fatalf("ClearAPIKey returned error: %v", err)
	}
	provider := providerByID(t, list, "jimeng")
	if provider.Configured || provider.Source != "none" {
		t.Fatalf("provider after clear = %#v, want unconfigured provider", provider)
	}
	output, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading fake jimeng args: %v", err)
	}
	if strings.TrimSpace(string(output)) != "logout" {
		t.Fatalf("jimeng args = %q, want logout", string(output))
	}
}

func TestSettingsClearLibTVAPIKeyRunsLogout(t *testing.T) {
	store := &memoryAPIKeyStore{values: map[string]string{"libtv": "oauth:old"}}
	settings := NewSettings(store)
	settings.SetGenerationCLIs([]string{"libtv"})
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.log")
	binPath := writeSettingsCLIFixture(t, "logout", argsPath)
	settings.SetLibTVCLIPaths(binPath, "")
	t.Cleanup(func() {
		if err := settings.cancelActiveProviderLogin(context.Background(), generation.ProviderLibTV); err != nil {
			t.Errorf("closing login fixture: %v", err)
		}
	})

	list, err := settings.ClearAPIKey(context.Background(), "libtv")
	if err != nil {
		t.Fatalf("ClearAPIKey returned error: %v", err)
	}
	provider := providerByID(t, list, "libtv")
	if provider.Configured || provider.Source != "none" {
		t.Fatalf("provider after clear = %#v, want unconfigured provider", provider)
	}
	output, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading fake libtv args: %v", err)
	}
	if strings.TrimSpace(string(output)) != "logout" {
		t.Fatalf("libtv args = %q, want logout", string(output))
	}
}

func TestSettingsBeginJimengLoginStoresOAuthMarkerWhenSessionExists(t *testing.T) {
	settings := NewSettings(&memoryAPIKeyStore{values: map[string]string{}})
	binPath := writeSettingsCLIFixture(t, "jimeng-existing", "")
	settings.SetJimengCLIPaths(binPath, "")

	result, err := settings.BeginJimengLogin(context.Background(), false)
	if err != nil {
		t.Fatalf("BeginJimengLogin returned error: %v", err)
	}
	if result.Login.Status != "completed" {
		t.Fatalf("login = %#v, want completed", result.Login)
	}
	provider := providerByID(t, APIKeyList{Providers: result.Providers}, "jimeng")
	if !provider.Configured || provider.Source != "settings" || provider.CredentialKind != "oauth" {
		t.Fatalf("jimeng provider = %#v, want configured oauth provider", provider)
	}
	if provider.Masked != "" {
		t.Fatalf("jimeng provider should not expose masked oauth marker: %#v", provider)
	}
}

func TestSettingsJimengHeadlessLoginReturnsChallengeAndCompletes(t *testing.T) {
	store := &memoryAPIKeyStore{values: map[string]string{}}
	settings := NewSettings(store)
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.log")
	binPath := writeSettingsCLIFixture(t, "jimeng-headless", argsPath)
	settings.SetJimengCLIPaths(binPath, "")

	result, err := settings.BeginJimengLogin(context.Background(), false)
	if err != nil {
		t.Fatalf("BeginJimengLogin returned error: %v", err)
	}
	if result.Login.Status != "pending" ||
		result.Login.VerificationURI != "https://example.test/device" ||
		result.Login.UserCode != "ABCD-EFGH" {
		t.Fatalf("login = %#v, want parsed browser challenge", result.Login)
	}
	if result.Login.DeviceCode != "device-123" {
		t.Fatalf("login device code = %q, want device-123", result.Login.DeviceCode)
	}
	provider := providerByID(t, APIKeyList{Providers: result.Providers}, "jimeng")
	if provider.Configured {
		t.Fatalf("provider = %#v, want unconfigured while challenge is pending", provider)
	}
	value, _, err := store.Get("jimeng")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value != "" {
		t.Fatalf("stored jimeng marker = %q, want empty before confirmation", value)
	}

	completed, err := settings.CompleteJimengLogin(context.Background(), result.Login.DeviceCode)
	if err != nil {
		t.Fatalf("CompleteJimengLogin returned error: %v", err)
	}
	if completed.Login.Status != "completed" {
		t.Fatalf("completed login = %#v, want completed", completed.Login)
	}
	provider = providerByID(t, APIKeyList{Providers: completed.Providers}, "jimeng")
	if !provider.Configured {
		t.Fatalf("provider = %#v, want configured after confirmation", provider)
	}
	output, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading fake jimeng args: %v", err)
	}
	wantArgs := "login --headless\nlogin checklogin --device_code=device-123 --poll=30"
	if strings.TrimSpace(string(output)) != wantArgs {
		t.Fatalf("jimeng args = %q, want %q", strings.TrimSpace(string(output)), wantArgs)
	}
}

func TestSettingsBeginLibTVLoginReturnsChallengeAndPersistsAfterCLICompletes(t *testing.T) {
	store := &memoryAPIKeyStore{values: map[string]string{}}
	settings := NewSettings(store)
	settings.SetGenerationCLIs([]string{"libtv"})
	binPath := writeSettingsCLIFixture(t, "libtv-success", "")
	settings.SetLibTVCLIPaths(binPath, "")
	t.Cleanup(func() {
		if err := settings.cancelActiveProviderLogin(context.Background(), generation.ProviderLibTV); err != nil {
			t.Errorf("closing login fixture: %v", err)
		}
	})

	result, err := settings.BeginLibTVLogin(context.Background(), false)
	if err != nil {
		t.Fatalf("BeginLibTVLogin returned error: %v", err)
	}
	if result.Login.Status != "pending" ||
		result.Login.VerificationURI != "https://libtv.example.test/login" {
		t.Fatalf("login = %#v, want parsed LibTV browser challenge", result.Login)
	}
	if result.Login.DeviceCode != "" {
		t.Fatalf("login device code = %q, want empty device code", result.Login.DeviceCode)
	}
	provider := providerByID(t, APIKeyList{Providers: result.Providers}, "libtv")
	if provider.Configured {
		t.Fatalf("provider = %#v, want unconfigured while challenge is pending", provider)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		value, _, err := store.Get("libtv")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if strings.HasPrefix(value, "oauth:") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("libtv oauth marker was not persisted after login command completed")
}

func TestSettingsLibTVPendingLoginCanBeReplacedAndClearedForRetry(t *testing.T) {
	store := &memoryAPIKeyStore{values: map[string]string{}}
	settings := NewSettings(store)
	settings.SetGenerationCLIs([]string{"libtv"})
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "args.log")
	binPath := writeSettingsCLIFixture(t, "libtv-pending", argsPath)
	settings.SetLibTVCLIPaths(binPath, "")
	t.Cleanup(func() {
		if err := settings.cancelActiveProviderLogin(context.Background(), generation.ProviderLibTV); err != nil {
			t.Errorf("closing login fixture: %v", err)
		}
	})

	first, err := settings.BeginLibTVLogin(context.Background(), false)
	if err != nil {
		t.Fatalf("first BeginLibTVLogin returned error: %v", err)
	}
	if first.Login.Status != "pending" {
		t.Fatalf("first login = %#v, want pending", first.Login)
	}
	second, err := settings.BeginLibTVLogin(context.Background(), false)
	if err != nil {
		t.Fatalf("second BeginLibTVLogin returned error: %v", err)
	}
	if second.Login.Status != "pending" {
		t.Fatalf("second login = %#v, want pending", second.Login)
	}
	if _, err := settings.ClearAPIKey(context.Background(), "libtv"); err != nil {
		t.Fatalf("ClearAPIKey returned error: %v", err)
	}

	third, err := settings.BeginLibTVLogin(context.Background(), false)
	if err != nil {
		t.Fatalf("third BeginLibTVLogin returned error: %v", err)
	}
	if third.Login.Status != "pending" {
		t.Fatalf("third login = %#v, want pending", third.Login)
	}
	if _, err := settings.ClearAPIKey(context.Background(), "libtv"); err != nil {
		t.Fatalf("second ClearAPIKey returned error: %v", err)
	}

	output, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("reading fake libtv args: %v", err)
	}
	wantArgs := "login web\nlogin web\nlogout\nlogin web\nlogout"
	if strings.TrimSpace(string(output)) != wantArgs {
		t.Fatalf("libtv args = %q, want %q", strings.TrimSpace(string(output)), wantArgs)
	}
}

func providerByID(t *testing.T, list APIKeyList, id string) APIKeyProvider {
	t.Helper()
	for _, provider := range list.Providers {
		if provider.ID == id {
			return provider
		}
	}
	t.Fatalf("provider %q not found", id)
	return APIKeyProvider{}
}

func modelPlatformExists(list ModelPlatformList, id string) bool {
	for _, platform := range list.Platforms {
		if platform.ID == id {
			return true
		}
	}
	return false
}

func apiKeyProviderExists(list APIKeyList, id string) bool {
	for _, provider := range list.Providers {
		if provider.ID == id {
			return true
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
