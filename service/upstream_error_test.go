package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/model"
)

func TestUpstreamErrorDetailsAndRedaction(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{`{"error":{"message":"first"},"detail":"second","message":"third","msg":"fourth"}`, "first"},
		{`{"detail":"second","message":"third","msg":"fourth"}`, "second"},
		{`{"message":"third","msg":"fourth"}`, "third"},
		{`{"msg":"fourth"}`, "fourth"},
		{`{"error":"{\"message\":\"nested\",\"request_id\":\"body-id\"}"}`, "nested"},
		{`"{\"detail\":\"{\\\"message\\\":\\\"nested string\\\"}\"}"`, "nested string"},
		{`{"detail":[{"msg":"field rejected"}]}`, "field rejected"},
		{`<html><body>upstream unavailable</body></html>`, "upstream unavailable"},
		{`{"request":{"prompt":"do not echo requests"}}`, "鉴权失败，接口没有提供有效错误详情"},
		{"", "鉴权失败，接口没有提供有效错误详情"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			err := NewUpstreamError([]byte(tc.body), 401, "创建", nil, nil)
			if err.Detail != tc.want || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "创建") {
				t.Fatal(err)
			}
		})
	}
	secret := "fixture-key+/\"&"
	encoded, _ := json.Marshal(secret)
	raw := "bad key " + secret + " " + string(encoded[1:len(encoded)-1]) + " Authorization: Bearer other-token; media=https://media.invalid/private?signature=signed-secret&token=grant-secret more https%3A%2F%2Fmedia.invalid%2Fprivate%3Ftoken%3Dgrant-secret data:image/png;base64,private-bytes"
	err := NewUpstreamError([]byte(raw), 403, "下载", http.Header{"X-Request-Id": {"request-123"}}, nil, secret)
	for _, value := range []string{secret, "other-token", "signed-secret", "grant-secret", "private-bytes", "media.invalid"} {
		if strings.Contains(err.Error(), value) {
			t.Fatal("secret leaked", value)
		}
	}
	if err.RequestID != "request-123" || !strings.Contains(err.Detail, "bad key") {
		t.Fatal(err)
	}
	id := NewUpstreamError([]byte(`{"error":"{\"message\":\"denied\",\"request_id\":\"body-id\"}"}`), 400, "轮询", nil, nil)
	if id.RequestID != "body-id" {
		t.Fatal(id)
	}
	long := NewUpstreamError([]byte(strings.Repeat("测试", 3000)+secret), 429, "创建", nil, nil, secret)
	if len([]rune(long.Detail)) > upstreamErrorTextLimit+1 || strings.Contains(long.Error(), secret) {
		t.Fatal("summary boundary/redaction")
	}
}

func TestTranslatedErrorStagesKeepStatusIDAndOriginalTask(t *testing.T) {
	for _, status := range []int{400, 401, 403, 429, 502} {
		for _, stage := range []string{"创建", "轮询", "下载"} {
			t.Run(fmt.Sprintf("%d/%s", status, stage), func(t *testing.T) {
				channel, vars := formFixture(t)
				calls := 0
				fixture := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					path, method := "/jobs", "POST"
					if stage != "创建" {
						path, method = "/jobs/original-id", "GET"
					}
					if stage == "下载" {
						path += "/content"
					}
					if r.URL.Path != path || r.Method != method || r.Header.Get("Authorization") != "Bearer fixture-key" {
						t.Error("request identity/auth changed", r.URL.Path, r.Method)
					}
					w.Header().Set("X-Request-ID", "fixture-request-id")
					w.WriteHeader(status)
					if status == 502 {
						fmt.Fprint(w, "upstream unavailable https://media.invalid/private?signature=private-grant fixture-key")
					} else {
						fmt.Fprint(w, `{"detail":"{\"error\":{\"message\":\"generation denied fixture-key https://media.invalid/private?signature=private-grant\"}}","id":"changed-id","task_id":"changed-task"}`)
					}
				}))
				defer fixture.Close()
				tr, err := NewParameterTranslation(channel, "fixture-video", vars, "video")
				if err != nil {
					t.Fatal(err)
				}
				snapshot := parameterTranslationSnapshot{Source: channel.ParameterTranslation, BaseURL: channel.BaseURL, Variables: vars, TaskIDs: []string{"original-id"}, Completed: map[string]ParameterTranslationResult{}}
				if stage == "下载" {
					snapshot.Completed["original-id"] = ParameterTranslationResult{ContentURL: channel.BaseURL + "/jobs/original-id/content"}
				}
				raw, _ := json.Marshal(snapshot)
				task := model.VideoTask{Model: "fixture-video", ParameterTranslationSnapshot: string(raw)}
				message := ""
				switch stage {
				case "创建":
					result, e := tr.Send(context.Background(), "video", false, "")
					if e == nil {
						t.Fatal("missing error")
					}
					message = e.Error() + result.ResponseBody
					if result.StatusCode != status {
						t.Fatal(result.StatusCode)
					}
				case "轮询":
					update, e := PollParameterTranslationVideo(context.Background(), task, channel)
					if e != nil {
						t.Fatal(e)
					}
					if update.Status != "failed" || update.StatusCode != status {
						t.Fatal(update)
					}
					message = update.Error + update.ErrorDetail + update.ResponseBody
				case "下载":
					response, e := ParameterTranslationVideoContent(context.Background(), task, channel)
					if response != nil {
						response.Body.Close()
					}
					if e == nil {
						t.Fatal("missing error")
					}
					var upstream *UpstreamError
					if !errors.As(e, &upstream) {
						t.Fatal(e)
					}
					message = e.Error() + upstream.Diagnostics()
				}
				for _, value := range []string{fmt.Sprintf("HTTP %d", status), stage, "fixture-request-id"} {
					if !strings.Contains(message, value) {
						t.Fatal("missing diagnostic", value, message)
					}
				}
				for _, value := range []string{"fixture-key", "private-grant"} {
					if strings.Contains(message, value) {
						t.Fatal("leaked response", value)
					}
				}
				if calls != 1 {
					t.Fatal("automatic retry", calls)
				}
			})
		}
	}
}

func TestUpstreamFullBodyAndTruncation(t *testing.T) {
	key := "fixture-key-with-special+/chars"
	body := map[string]any{
		"error":         map[string]any{"message": "permission denied", "extra": strings.Repeat("detail ", 1000)},
		"authorization": "Bearer opaque-secret", "api_key": "other-secret", "password": "other-password",
		"nested":  `{"cookie":"hidden-cookie","message":"still visible"}`,
		"links":   []any{"https://media.invalid/file?signature=hidden-signature", "/api/files/a?token=relative-grant"},
		"encoded": url.QueryEscape(key), "max_tokens": 2048, "counter": json.Number("9223372036854775807"), "trailing": "last diagnostic field",
	}
	raw, _ := json.Marshal(body)
	failure := NewUpstreamError(raw, 401, "创建", http.Header{"X-Request-Id": {"req-full"}}, nil, key)
	if failure.Truncated || !strings.Contains(failure.Body, "last diagnostic field") || !strings.Contains(failure.Body, "9223372036854775807") || !strings.Contains(failure.Body, `"max_tokens": 2048`) || !strings.Contains(failure.Body, strings.Repeat("detail ", 1000)) {
		t.Fatal("full body lost")
	}
	for _, secret := range []string{key, url.QueryEscape(key), "opaque-secret", "other-secret", "other-password", "hidden-cookie", "hidden-signature", "relative-grant"} {
		if strings.Contains(failure.Diagnostics(), secret) {
			t.Fatalf("secret leaked: %s", secret)
		}
	}
	text := strings.Repeat("a", UpstreamErrorBodyLimit-8) + key
	clipped := NewUpstreamError([]byte(text), 502, "下载", nil, nil, key)
	if !clipped.Truncated || !strings.Contains(clipped.Body, "已截断") || strings.Contains(clipped.Body, key[:8]) {
		t.Fatal("truncation flag/partial secret")
	}
	exact := NewUpstreamError([]byte(strings.Repeat("x", UpstreamErrorBodyLimit)), 500, "轮询", nil, nil)
	if exact.Truncated || len(exact.Body) != UpstreamErrorBodyLimit {
		t.Fatal("boundary must preserve exact-size body")
	}
}
