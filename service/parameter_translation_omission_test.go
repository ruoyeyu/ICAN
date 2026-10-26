package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
)

func TestFormOmissionCapturedRequests(t *testing.T) {
	source, err := os.ReadFile("testdata/channel-template-omission.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"json", "urlencoded", "formData"} {
		for _, value := range []any{nil, "", "auto", "720p", " auto ", "AUTO", 0, false} {
			t.Run(fmt.Sprintf("%s/%v", format, value), func(t *testing.T) {
				calls := 0
				fixture := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.URL.Path != "/jobs" || r.Header.Get("Authorization") != "Bearer fixture-key" {
						t.Error("request routing/auth changed")
					}
					body := map[string]any{}
					if format == "json" {
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
					} else {
						if format == "formData" {
							if err := r.ParseMultipartForm(1 << 20); err != nil {
								t.Error(err)
							}
							defer r.MultipartForm.RemoveAll()
						} else if err := r.ParseForm(); err != nil {
							t.Error(err)
						}
						for key, values := range r.PostForm {
							body[key] = values[0]
						}
					}
					omitted := value == nil || value == "" || value == "auto"
					_, sent := body["resolution"]
					if sent == omitted || r.URL.Query().Has("resolution") == omitted {
						t.Error("omission not applied to body/query", body, r.URL.Query())
					}
					if !omitted {
						if fmt.Sprint(body["resolution"]) != fmt.Sprint(value) || r.URL.Query().Get("resolution") != fmt.Sprint(value) {
							t.Error("value changed", body, r.URL.Query())
						}
					}
					for _, key := range []string{"fixed_auto", "explicit_zero", "explicit_false", "empty_list", "empty_string", "empty_null"} {
						if _, ok := body[key]; ok {
							t.Error("omitted field sent", key)
						}
					}
					for key, expected := range map[string]any{"zero": float64(0), "disabled": false, "fixed_zero": float64(0), "fixed_false": false, "string_zero": "0", "legacy": "auto", "model": "new-video", "prompt": " exact {channelKey} "} {
						if format != "json" {
							expected = fmt.Sprint(expected)
						}
						if !reflect.DeepEqual(body[key], expected) {
							t.Errorf("%s changed: %#v != %#v", key, body[key], expected)
						}
					}
					fmt.Fprint(w, `{"url":"https://media.invalid/result"}`)
				}))
				defer fixture.Close()
				channel := model.ModelChannel{BaseURL: "https://upstream.invalid", APIKey: "fixture-key", ParameterTranslation: strings.ReplaceAll(string(source), `format:"json"`, `format:"`+format+`"`)}
				vars := map[string]any{"resolution": value, "count": 0, "generateAudio": false, "quality": "auto", "rawPrompt": " exact {channelKey} "}
				if value == nil {
					delete(vars, "resolution")
				}
				tr, err := NewParameterTranslation(channel, "new-video", vars, "video")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := tr.Send(context.Background(), "video", false, ""); err != nil {
					t.Fatal(err)
				}
				if calls != 1 {
					t.Fatal("unexpected request count", calls)
				}
			})
		}
	}
}

func TestTemplateOmissionValidationAndCatalogRoundtrip(t *testing.T) {
	raw, err := os.ReadFile("../web/scripts/fixtures/channel-template-omission.json")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseChannelTemplate(raw)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ParseChannelTemplate(encoded)
	if err != nil || !reflect.DeepEqual(parsed.Authoring.Defaults[0].Fields, restored.Authoring.Defaults[0].Fields) {
		t.Fatal("template roundtrip changed omission rules", err)
	}
	old := config.Cfg.APITemplateDir
	config.Cfg.APITemplateDir = filepath.Join(t.TempDir(), "catalog")
	defer func() { config.Cfg.APITemplateDir = old }()
	saved, err := SaveChannelTemplate("", "", raw)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := GetChannelTemplate(saved.ID)
	if err != nil || !reflect.DeepEqual(parsed.Authoring.Defaults[0].Fields, loaded.Template.Authoring.Defaults[0].Fields) {
		t.Fatal("catalog changed rules", err)
	}
	for _, invalid := range []any{"auto", nil, []any{nil}, []any{map[string]any{}}, []any{[]any{}}} {
		var value map[string]any
		json.Unmarshal(raw, &value)
		field := value["authoring"].(map[string]any)["defaults"].([]any)[0].(map[string]any)["fields"].([]any)[0].(map[string]any)
		field["omitValues"] = invalid
		changed, _ := json.Marshal(value)
		if _, err := ParseChannelTemplate(changed); err == nil {
			t.Fatal("invalid omission accepted", invalid)
		}
	}
}
