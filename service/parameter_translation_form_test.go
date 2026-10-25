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

func formFixture(t *testing.T) (model.ModelChannel, map[string]any) {
	t.Helper()
	source, err := os.ReadFile("testdata/channel-template-v1.js")
	if err != nil {
		t.Fatal(err)
	}
	return model.ModelChannel{BaseURL: "https://upstream.invalid", APIKey: "fixture-key", ParameterTranslation: string(source)}, map[string]any{"model": "fixture-video", "rawPrompt": "  exact {channelKey}\n", "rawSize": "16:9", "rawSeconds": "2", "seconds": 2, "resolution": "720p", "images": []string{"https://media.invalid/signed"}, "firstFrame": "", "lastFrame": "", "videos": []string{}, "audios": []string{}, "mode": "first_last_frame"}
}
func TestFormFixtureVideoPollResumeAndAuthenticatedDownload(t *testing.T) {
	for _, redirect := range []string{"", "https://other.invalid/leak", "https://upstream.invalid/redirect"} {
		t.Run(redirect, func(t *testing.T) {
			channel, vars := formFixture(t)
			posts, gets, downloads := 0, 0, 0
			fixture := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Fatal("authentication missing")
				}
				switch r.URL.Path {
				case "/jobs":
					posts++
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					for k, v := range map[string]any{"model": "fixture-video", "prompt": vars["rawPrompt"], "aspect_ratio": "16:9", "seconds": "2", "resolution": "720p", "image_url": "https://media.invalid/signed"} {
						if body[k] != v {
							t.Fatalf("%s changed: %#v", k, body[k])
						}
					}
					fmt.Fprint(w, `{"id":"job"}`)
				case "/jobs/job":
					statuses := []string{"queued", "unknown", "in_progress", "completed"}
					status := statuses[min(gets, 3)]
					gets++
					fmt.Fprintf(w, `{"status":%q,"progress":37,"url":"https://preview.invalid/not-the-download"}`, status)
				case "/jobs/job/content":
					downloads++
					if r.Method != "GET" || r.ContentLength != 0 || r.Header.Get("Content-Type") != "" {
						t.Fatal("request body leaked")
					}
					if redirect != "" {
						w.Header().Set("Location", redirect)
						w.WriteHeader(302)
					} else {
						w.Header().Set("Content-Type", "video/mp4")
						fmt.Fprint(w, "fixture-video-bytes")
					}
				default:
					t.Fatalf("unexpected request: %s", r.URL)
				}
			}))
			defer fixture.Close()
			tr, err := NewParameterTranslation(channel, "fixture-video", vars)
			if err != nil {
				t.Fatal(err)
			}
			started, err := tr.Send(context.Background(), "video", false, "")
			if err != nil {
				t.Fatal(err)
			}
			task := model.VideoTask{Model: "fixture-video", ParameterTranslationSnapshot: tr.Snapshot(ParameterTranslationInput{Kind: "video", Variables: vars}, started)}
			for i := 0; i < 4; i++ {
				// Reconstruct the task from persisted JSON for every poll, as after restart.
				raw, _ := json.Marshal(task)
				json.Unmarshal(raw, &task)
				update, err := PollParameterTranslationVideo(context.Background(), task, channel)
				if err != nil {
					t.Fatal(err)
				}
				expected := "processing"
				if i == 3 {
					expected = "completed"
				}
				if update.Status != expected {
					t.Fatalf("state %d: %+v", i, update)
				}
				task.ParameterTranslationSnapshot = update.ParameterTranslationSnapshot
			}
			response, err := ParameterTranslationVideoContent(context.Background(), task, channel)
			if redirect != "" {
				if err == nil {
					response.Body.Close()
					t.Fatal("redirect allowed")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				data, _ := io.ReadAll(response.Body)
				response.Body.Close()
				if string(data) != "fixture-video-bytes" {
					t.Fatal("content mismatch")
				}
			}
			if posts != 1 || gets != 4 || downloads != 1 {
				t.Fatalf("posts=%d gets=%d downloads=%d", posts, gets, downloads)
			}
		})
	}
}
func TestFormMetadataRejectsInvalidInputsBeforeSubmission(t *testing.T) {
	channel, vars := formFixture(t)
	for _, name := range []string{"images", "firstFrame", "lastFrame", "videos", "audios"} {
		for _, bad := range []string{"blob:local", "data:image/png;base64,aaaa", "http://127.0.0.1/private", "http://169.254.169.254/latest"} {
			next := map[string]any{}
			for k, v := range vars {
				next[k] = v
			}
			next[name] = []string{bad}
			if name == "firstFrame" || name == "lastFrame" {
				next[name] = bad
			}
			if _, err := NewParameterTranslation(channel, "fixture-video", next); err == nil {
				t.Fatal("invalid media accepted", name, bad)
			}
		}
	}
	vars["rawSeconds"] = "3"
	if _, err := NewParameterTranslation(channel, "fixture-video", vars); err == nil {
		t.Fatal("undeclared duration accepted")
	}
	vars["rawSeconds"] = "2"
	vars["resolution"] = "custom-value"
	if _, err := NewParameterTranslation(channel, "fixture-video", vars); err != nil {
		t.Fatal("custom option rejected", err)
	}
	for _, value := range []string{`(()=>({version:1,parameters:{},media:{}}))()`, `{"version":2,"parameters":{},"media":{}}`, `{"version":1,"parameters":{"guessed":{}},"media":{}}`} {
		_, err := ParameterTranslationModels(`{models:{sample:{inputs:` + value + `,POST:{url:"/jobs"}}}}`)
		if err == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
	// Keep legacy implicit /videos/id/content behavior intact.
	legacy := strings.Replace(channel.ParameterTranslation, `content:{url:"/jobs/{taskId}/content"},`, "", 1)
	if _, err := ParameterTranslationModels(legacy); err != nil {
		t.Fatal(err)
	}
}

func TestFormAuthenticationAndCrossOriginRedirect(t *testing.T) {
	channel, vars := formFixture(t)
	if ParameterTranslationWithoutKey(channel.ParameterTranslation, "fixture-video") {
		t.Fatal("key requirement lost")
	}
	none := strings.ReplaceAll(channel.ParameterTranslation, `"authentication":"key"`, `"authentication":"none"`)
	if !ParameterTranslationWithoutKey(none, "fixture-video") || ParameterTranslationWithoutKey(none, "other") {
		t.Fatal("keyless declaration not scoped to exact model")
	}
	calls := 0
	fixture := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "https://other.invalid/leak")
		w.WriteHeader(307)
	}))
	defer fixture.Close()
	tr, err := NewParameterTranslation(channel, "fixture-video", vars)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Send(context.Background(), "video", false, ""); err == nil || !strings.Contains(err.Error(), "跨来源") {
		t.Fatal("cross-origin request redirect allowed", err)
	}
	if calls != 1 {
		t.Fatal("redirect followed")
	}
}

func TestFormRejectsUndeclaredMediaBeforeCharging(t *testing.T) {
	source := `{models:{sample:{inputs:{"version":1,"parameters":{},"media":{}},POST:{url:"/jobs",body:{model,prompt}},result:"url"}}}`
	for _, name := range []string{"images", "firstFrame", "lastFrame", "videos", "audios"} {
		var value any = []string{"https://media.invalid/owned"}
		if name == "firstFrame" || name == "lastFrame" {
			value = "https://media.invalid/owned"
		}
		if _, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://upstream.invalid", ParameterTranslation: source}, "sample", map[string]any{name: value}); err == nil {
			t.Fatal("undeclared input silently dropped", name)
		}
	}
}

func TestRawVideoParametersStayLiteralAcrossEncodings(t *testing.T) {
	for _, format := range []string{"json", "urlencoded", "formData"} {
		source := `{headers:{Authorization:"Bearer {channelKey}"},models:{sample:{POST:{url:"/jobs",format:"` + format + `",params:{p:rawPrompt},body:{prompt:rawPrompt,seconds:rawSeconds,resolution,mode,model}},result:"url"}}}`
		vars := map[string]any{"rawPrompt": " literal {channelKey} {taskId} ", "rawSeconds": "{channelKey}", "resolution": "custom-{taskId}", "mode": "custom-{channelKey}"}
		tr, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://upstream.invalid", APIKey: "fixture-secret", ParameterTranslation: source}, "sample", vars)
		if err != nil {
			t.Fatal(err)
		}
		req, _, err := tr.request(context.Background(), false, "")
		if err != nil {
			t.Fatal(err)
		}
		if req.URL.Query().Get("p") != vars["rawPrompt"] || req.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Fatal("query/auth changed")
		}
		got := map[string]string{}
		switch format {
		case "json":
			json.NewDecoder(req.Body).Decode(&got)
		case "urlencoded":
			req.ParseForm()
			for k := range vars {
				field := k
				if k == "rawPrompt" {
					field = "prompt"
				}
				if k == "rawSeconds" {
					field = "seconds"
				}
				got[field] = req.PostForm.Get(field)
			}
		case "formData":
			req.ParseMultipartForm(1 << 20)
			defer req.MultipartForm.RemoveAll()
			for k, v := range req.MultipartForm.Value {
				got[k] = v[0]
			}
		}
		for k, v := range vars {
			field := k
			if k == "rawPrompt" {
				field = "prompt"
			}
			if k == "rawSeconds" {
				field = "seconds"
			}
			if got[field] != v {
				t.Fatal(format, field, got[field], v)
			}
		}
	}
}
