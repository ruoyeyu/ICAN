package service

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

func TestChannelPermissionsPreserveStorageAndCredentials(t *testing.T) {
	const marker = "ICAN_SETTINGS_PERMISSIONS_CHILD"
	if os.Getenv(marker) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestChannelPermissionsPreserveStorageAndCredentials$", "-test.timeout=30s")
		cmd.Env = append(os.Environ(), marker+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated settings test: %v\n%s", err, output)
		}
		return
	}
	config.Cfg = config.Config{PublicOrigin: "https://ican.invalid", DataEncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", StorageDriver: "sqlite", DatabaseDSN: ":memory:", AILogDir: t.TempDir()}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	defer conn.Close()
	source := `{models:{sample:{references:{format:"url",max:6},POST:{url:"/generate",body:{rawPrompt}}}}}`
	initial := normalizeSettings(model.Settings{Private: model.PrivateSetting{Channels: []model.ModelChannel{{ID: "stable-cloud", Name: "fixture", Protocol: "custom", BaseURL: "https://provider.invalid", APIKey: "cloud-secret", Enabled: true, Weight: 1, Models: []string{"sample"}, ParameterTranslation: source}}}})
	if _, err = repository.SaveSettings(initial, "fixture"); err != nil {
		t.Fatal(err)
	}
	form, err := AdminSettings()
	if err != nil {
		t.Fatal(err)
	}
	enabled := true
	form.Public.ModelChannel.AllowCustomChannel = &enabled
	form.Public.ModelChannel.AllowUserRemoteChannel = &enabled
	if _, err = SaveSettings(form); err != nil {
		t.Fatal(err)
	}
	saved, err := repository.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Private.Storage.Providers, initial.Private.Storage.Providers) {
		t.Fatal("storage changed")
	}
	if saved.Private.Channels[0].ID != "stable-cloud" || saved.Private.Channels[0].APIKey != "cloud-secret" || saved.Private.Channels[0].ParameterTranslation != source {
		t.Fatal("channel or secret changed")
	}
	user := model.AuthUser{ID: "ordinary", Role: model.UserRoleUser}
	if !UserCanUseRemoteModelChannel(user) {
		t.Fatal("cloud channels not available")
	}
	ctx := WithUser(context.Background(), user)
	raw := json.RawMessage(`{"localChannels":[{"id":"stable-personal","name":"own","protocol":"custom","baseUrl":"https://own.invalid","apiKey":"personal-secret","models":["sample"]}]}`)
	if _, err = SaveCurrentUserModelConfig(ctx, raw); err != nil {
		t.Fatal(err)
	}
	personal, err := SelectUserLocalModelChannelForModel(user.ID, "sample", "stable-personal")
	if err != nil || personal.APIKey != "personal-secret" {
		t.Fatal("personal channel unavailable", err)
	}
	public, err := PublicSettings()
	if err != nil {
		t.Fatal(err)
	}
	meta := public.ModelChannel.Channels[0].ParameterTranslationReferences["sample"]
	if meta.Format != "url" || meta.Max == nil || *meta.Max != 6 {
		t.Fatal("reference metadata not published")
	}
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), "cloud-secret") || strings.Contains(string(encoded), source) {
		t.Fatal("public settings leaked credentials or template")
	}
	disabled := false
	form.Public.ModelChannel.AllowCustomChannel = &disabled
	form.Public.ModelChannel.AllowUserRemoteChannel = &disabled
	if _, err = SaveSettings(form); err != nil {
		t.Fatal(err)
	}
	if UserCanUseRemoteModelChannel(user) || RequireCustomChannels() == nil {
		t.Fatal("disabled channels allowed")
	}
	if _, err = SaveCurrentUserModelConfig(ctx, raw); err == nil {
		t.Fatal("custom config saved while disabled")
	}
	if err = validateEnabledStorageProviderTypes([]model.StorageProvider{{ID: LocalMediaProvider, Type: "s3"}}); err == nil {
		t.Fatal("local provider identity rewritten")
	}
}
