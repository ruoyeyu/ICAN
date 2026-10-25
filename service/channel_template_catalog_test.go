package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/tigerowo/infinite-canvas/config"
)

func templateFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("../web/scripts/fixtures/channel-template-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTemplateValidation(t *testing.T) {
	raw := templateFixture(t)
	if _, err := ParseChannelTemplate(raw); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"key":                 func(v map[string]any) { v["apiKey"] = "private-key" },
		"missing-address":     func(v map[string]any) { delete(v, "baseUrl") },
		"null-models":         func(v map[string]any) { v["models"] = nil },
		"unsupported-version": func(v map[string]any) { v["version"] = 2 },
		"address-credentials": func(v map[string]any) { v["baseUrl"] = "https://user:secret@api.invalid" },
		"address-query":       func(v map[string]any) { v["baseUrl"] = "https://api.invalid?token=secret" },
		"code":                func(v map[string]any) { v["parameterTranslation"] = "process.exit()" },
		"missing-boolean": func(v map[string]any) {
			delete(v["authoring"].(map[string]any)["models"].([]any)[0].(map[string]any)["poll"].(map[string]any), "enabled")
		},
		"nested-key": func(v map[string]any) {
			v["authoring"].(map[string]any)["models"].([]any)[0].(map[string]any)["fields"].([]any)[0] = map[string]any{"target": "extra", "source": "literal", "value": `{"apiKey":"hidden"}`, "string": false, "optional": false, "when": "always", "location": "body"}
		},
		"prototype": func(v map[string]any) {
			v["authoring"].(map[string]any)["models"].([]any)[0].(map[string]any)["fields"].([]any)[0].(map[string]any)["target"] = "__proto__.x"
		},
		"cross-origin-path": func(v map[string]any) {
			v["authoring"].(map[string]any)["models"].([]any)[0].(map[string]any)["path"] = "//other.invalid/jobs"
		},
		"missing-task-id": func(v map[string]any) {
			v["authoring"].(map[string]any)["models"].([]any)[0].(map[string]any)["poll"].(map[string]any)["taskId"] = ""
		},
	} {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			json.Unmarshal(raw, &v)
			mutate(v)
			changed, _ := json.Marshal(v)
			if _, err := ParseChannelTemplate(changed); err == nil {
				t.Fatal("invalid template accepted")
			}
		})
	}
	var v map[string]any
	json.Unmarshal(raw, &v)
	v["instructions"] = "<script>plain text only</script>"
	changed, _ := json.Marshal(v)
	if _, err := ParseChannelTemplate(changed); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateCatalogLifecycle(t *testing.T) {
	old := config.Cfg.APITemplateDir
	config.Cfg.APITemplateDir = filepath.Join(t.TempDir(), "catalog")
	defer func() { config.Cfg.APITemplateDir = old }()
	list, err := ListChannelTemplates()
	if err != nil || len(list.Items) != 0 {
		t.Fatalf("empty catalog: %v", err)
	}
	raw := templateFixture(t)
	one, err := SaveChannelTemplate("", "", raw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(config.Cfg.APITemplateDir, one.ID+".json")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private file permissions")
	}
	// A separately written file is discovered immediately, without a restart/cache.
	if err := os.WriteFile(filepath.Join(config.Cfg.APITemplateDir, "manual.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	list, err = ListChannelTemplates()
	if err != nil || len(list.Items) != 2 || list.Items[0].Template != nil {
		t.Fatal("dynamic catalog list")
	}
	outside := filepath.Join(t.TempDir(), "secret.json")
	os.WriteFile(outside, raw, 0600)
	os.Symlink(outside, filepath.Join(config.Cfg.APITemplateDir, "escape.json"))
	os.Symlink(one.ID+".json", filepath.Join(config.Cfg.APITemplateDir, "inside-link.json"))
	syscall.Mkfifo(filepath.Join(config.Cfg.APITemplateDir, "pipe.json"), 0600)
	os.WriteFile(filepath.Join(config.Cfg.APITemplateDir, "bad.json"), []byte(`{"apiKey":"must-not-leak"}`), 0600)
	list, err = ListChannelTemplates()
	if err != nil || len(list.Items) != 2 || len(list.Errors) != 4 || strings.Contains(strings.Join(list.Errors, ""), "must-not-leak") {
		t.Fatalf("invalid-file handling: %+v %v", list, err)
	}
	for _, id := range []string{"../secret", "/secret", "escape", "inside-link", "pipe"} {
		if _, err := GetChannelTemplate(id); err == nil {
			t.Fatal("unsafe file accepted", id)
		}
	}
	if _, err := SaveChannelTemplate(one.ID, "stale", raw); err == nil {
		t.Fatal("stale overwrite")
	}
	updated := *one.Template
	updated.Name = "Updated fixture"
	changed, _ := json.Marshal(updated)
	two, err := SaveChannelTemplate(one.ID, one.Revision, changed)
	if err != nil || two.Revision == one.Revision {
		t.Fatalf("edit: %v", err)
	}
	if err := DeleteChannelTemplate(one.ID, one.Revision); err == nil {
		t.Fatal("stale delete")
	}
	if err := DeleteChannelTemplate(one.ID, two.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file not deleted")
	}
	if _, err := GetChannelTemplate("manual"); err != nil {
		t.Fatal("other supplier affected")
	}
	files, _ := os.ReadDir(config.Cfg.APITemplateDir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".tmp") {
			t.Fatal("temporary file left")
		}
	}
}
