package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/tigerowo/infinite-canvas/model"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSupplierDefaultRulesWireAndRecovery(t *testing.T) {
	source, err := os.ReadFile("testdata/channel-template-v2.js")
	if err != nil {
		t.Fatal(err)
	}
	portable, err := os.ReadFile("../web/scripts/fixtures/channel-template-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	template, err := ParseChannelTemplate(portable)
	if err != nil || len(template.Authoring.Defaults) != 3 {
		t.Fatalf("portable: %v", err)
	}
	channel := model.ModelChannel{BaseURL: "https://upstream.invalid", Protocol: "custom", APIKey: "fixture-key", ParameterTranslation: string(source)}
	variables := map[string]any{"model": "wrong-input-model", "rawPrompt": "exact prompt", "rawSize": "9:16", "rawSeconds": "3", "resolution": "custom-resolution", "messages": []any{map[string]any{"role": "user", "content": "hello"}}}
	posts, polls := 0, 0
	fixture := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Fatal("lost credential")
		}
		if r.Method == "POST" {
			posts++
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			expected := "newly-added"
			if r.URL.Path == "/special-jobs" {
				expected = "fixture-special"
			}
			if body["model"] != expected {
				t.Fatalf("model changed: %v", body)
			}
			switch r.URL.Path {
			case "/jobs", "/special-jobs":
				if body["aspect_ratio"] != "9:16" || body["resolution"] != "custom-resolution" {
					t.Fatal(body)
				}
				fmt.Fprint(w, `{"id":"job"}`)
			case "/pictures":
				fmt.Fprint(w, `{"data":[{"url":"https://media.invalid/picture"}]}`)
			case "/text":
				if body["messages"] == nil {
					t.Fatal("messages dropped")
				}
				fmt.Fprint(w, `{"reply":{"role":"assistant","content":"hello","tool_calls":[]}}`)
			default:
				t.Fatalf("wrong path %s", r.URL)
			}
		} else if r.URL.Path == "/jobs/job/content" {
			w.Header().Set("Content-Type", "video/mp4")
			fmt.Fprint(w, "fixture-video-bytes")
		} else if r.URL.Path == "/jobs/job" {
			polls++
			status := "unknown"
			if polls > 1 {
				status = "completed"
			}
			fmt.Fprintf(w, `{"status":%q}`, status)
		} else {
			t.Fatal(r.URL)
		}
	}))
	defer fixture.Close()
	for _, kind := range []string{"text", "image", "video"} {
		tr, err := NewParameterTranslation(channel, "newly-added", variables, kind)
		if err != nil || tr == nil {
			t.Fatalf("%s: %v", kind, err)
		}
		result, err := tr.Send(context.Background(), kind, false, "")
		if err != nil {
			t.Fatal(err)
		}
		if kind == "text" && result.TextResult.(map[string]any)["content"] != "hello" {
			t.Fatal(result)
		}
		if kind == "video" {
			task := model.VideoTask{Model: "newly-added", ParameterTranslationSnapshot: tr.Snapshot(ParameterTranslationInput{Kind: kind, Variables: variables}, result)}
			old := channel.ParameterTranslation
			channel.ParameterTranslation = ""
			for i := 0; i < 2; i++ {
				update, err := PollParameterTranslationVideo(context.Background(), task, channel)
				if err != nil {
					t.Fatal(err)
				}
				expected := "processing"
				if i == 1 {
					expected = "completed"
				}
				if update.Status != expected {
					t.Fatal(update)
				}
				task.ParameterTranslationSnapshot = update.ParameterTranslationSnapshot
			}
			response, err := ParameterTranslationVideoContent(context.Background(), task, channel)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if string(body) != "fixture-video-bytes" {
				t.Fatal("download")
			}
			channel.ParameterTranslation = old
		}
	}
	if posts != 3 || polls != 2 {
		t.Fatalf("unexpected retries posts=%d polls=%d", posts, polls)
	}
	if _, err := NewParameterTranslation(channel, "fixture-special", variables, "video"); err == nil {
		t.Fatal("model-specific options lost")
	}
	variables["rawSeconds"] = "7"
	tr, err := NewParameterTranslation(channel, "fixture-special", variables, "video")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tr.Send(context.Background(), "video", false, ""); err != nil {
		t.Fatal(err)
	}
	if tr, err := NewParameterTranslation(channel, "newly-added", variables, "audio"); err != nil || tr != nil {
		t.Fatalf("mixed types: %v", err)
	}
	public := ParameterTranslationPublicRules(channel.ParameterTranslation, []string{"newly-added", "fixture-special"})
	encoded, _ := json.Marshal(public)
	if len(public["video"]) != 2 || len(public["audio"]) != 0 || strings.Contains(string(encoded), channel.APIKey) || strings.Contains(string(encoded), "/jobs") {
		t.Fatal("public metadata")
	}
	for _, bad := range []string{`{models:{},defaults:{video:{}}}`, `{models:{},defaults:{video:{kind:"image",POST:{url:"/x"}}}}`, `{models:{a:{POST:{url:"/x"}}},defaults:{video:{POST:{url:"/x"}}}}`} {
		if _, err := ParameterTranslationModels(bad); err == nil {
			t.Fatal("accepted invalid shared rules")
		}
	}
}

func TestTextRuleRejectsUnsupportedCallsBeforeSending(t *testing.T) {
	for _, source := range []string{
		`{models:{},defaults:{text:{POST:{url:"/text",body:{stream:true}}}}}`,
		`{models:{},defaults:{text:{POST:{url:"/text",responseType:"blob"}}}}`,
		`{models:{},defaults:{text:{POST:{url:"/text"},poll:{taskId:"id",GET:{url:"/task/{taskId}"},status:"status",success:["done"]}}}}`,
	} {
		if _, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://must-not-be-called.invalid", ParameterTranslation: source}, "new", nil, "text"); err == nil {
			t.Fatal("unsupported text call accepted")
		}
	}
	for _, body := range []string{`{"_parameterTranslation":{"kind":"text","variables":{"messages":"wrong"}}}`, `{"_parameterTranslation":{"kind":"image","variables":{"request":{}}}}`} {
		if _, err := ReadParameterTranslationInput([]byte(body)); err == nil {
			t.Fatal("invalid variables accepted")
		}
	}
}

func TestSharedRuleUpstreamUnauthorizedDoesNotChangeMethodOrRetry(t *testing.T) {
	calls := 0
	fixture := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/configured" || r.Method != "POST" {
			t.Fatal("fell back to another request")
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"fixture unauthorized"}}`)
	}))
	defer fixture.Close()
	channel := model.ModelChannel{BaseURL: "https://upstream.invalid", APIKey: "fixture-key", Protocol: "custom", ParameterTranslation: `{models:{},defaults:{video:{POST:{url:"/configured",body:{model}},result:"url"}}}`}
	tr, err := NewParameterTranslation(channel, "new-model", nil, "video")
	if err != nil {
		t.Fatal(err)
	}
	result, err := tr.Send(context.Background(), "video", false, "")
	if err == nil || result.StatusCode != http.StatusUnauthorized || calls != 1 {
		t.Fatalf("status=%d calls=%d err=%v", result.StatusCode, calls, err)
	}
}
