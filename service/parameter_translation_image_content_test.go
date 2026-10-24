package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/model"
)

func TestParameterTranslationImageContentIsExplicitAndSameOrigin(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGZkAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, kind, resultURL string
		content              []byte
		download, wantError  bool
	}{
		{"authenticated image", "image", "https://upstream.invalid/v1/videos/task/content?token=result", png, true, false},
		{"ordinary image URL", "image", "https://upstream.invalid/files/image.png", nil, false, false},
		{"cross origin", "image", "https://other.invalid/v1/videos/task/content", nil, false, false},
		{"userinfo", "image", "https://user@upstream.invalid/v1/videos/task/content", nil, false, false},
		{"HTML disguised as image", "image", "https://upstream.invalid/v1/videos/task/content", []byte("<!doctype html><html>not an image</html>"), true, true},
		{"video remains deferred", "video", "https://upstream.invalid/v1/videos/task/content", nil, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := []string{}
			upstream := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.Method+" "+r.URL.Path)
				if r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Fatalf("channel authorization missing at %s", r.URL.Path)
				}
				if r.Method == http.MethodPost {
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"url":%q}`, test.resultURL)
					return
				}
				if !test.download || r.URL.Path != "/v1/videos/task/content" {
					t.Fatalf("unexpected content request: %s", r.URL)
				}
				if r.URL.Query().Get("keep") != "base" || (strings.Contains(test.resultURL, "token=result") && r.URL.Query().Get("token") != "result") {
					t.Fatal("content URL query did not preserve explicit values")
				}
				var body []byte
				if r.Body != nil {
					body, _ = io.ReadAll(r.Body)
				}
				if len(body) != 0 || r.Header.Get("Content-Length") != "" {
					t.Fatal("submission body leaked into content GET")
				}
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(test.content)
			}))
			defer upstream.Close()
			channel := model.ModelChannel{BaseURL: upstream.URL + "/v1?token=base&keep=base", APIKey: "fixture-key", ParameterTranslation: `{models:{sample:{POST:{url:"/generate",body:{prompt}},result:"url"}}}`}
			translation, err := NewParameterTranslation(channel, "sample", map[string]any{"prompt": "unchanged prompt"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := translation.Send(context.Background(), test.kind, false, "")
			if (err != nil) != test.wantError {
				t.Fatalf("error=%v wantError=%v", err, test.wantError)
			}
			wantCalls := []string{"POST /v1/generate"}
			if test.download {
				wantCalls = append(wantCalls, "GET /v1/videos/task/content")
			}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("requests=%v want=%v", calls, wantCalls)
			}
			if test.wantError {
				return
			}
			if test.kind == "video" {
				if result.ContentURL != test.resultURL || len(result.URLs) != 0 {
					t.Fatalf("video behavior changed: %+v", result)
				}
			} else if test.download {
				want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				if !reflect.DeepEqual(result.URLs, []string{want}) {
					t.Fatal("authenticated image result was not preserved as PNG")
				}
			} else if !reflect.DeepEqual(result.URLs, []string{test.resultURL}) {
				t.Fatalf("ordinary result changed: %v", result.URLs)
			}
		})
	}
}

func TestParameterTranslationImageDoesNotGuessContentEndpoint(t *testing.T) {
	calls := 0
	upstream := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/videos/task" {
			t.Fatalf("unconfigured request: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"completed"}`)
	}))
	defer upstream.Close()
	translation, err := NewParameterTranslation(model.ModelChannel{BaseURL: upstream.URL + "/v1", ParameterTranslation: `{models:{sample:{POST:{url:"/videos"},poll:{taskId:"id",GET:{url:"/videos/{taskId}"},status:"status",success:["completed"]}}}}`}, "sample", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := translation.Send(context.Background(), "image", true, "task"); err == nil || calls != 1 {
		t.Fatalf("image guessed an undeclared content endpoint: calls=%d error=%v", calls, err)
	}
}
