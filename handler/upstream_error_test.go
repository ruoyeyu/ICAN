package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
	"github.com/tigerowo/infinite-canvas/service"
)

func TestUpstreamDiagnosticsOwnership(t *testing.T) {
	const marker = "UPSTREAM_DIAGNOSTICS_CHILD"
	if os.Getenv(marker) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUpstreamDiagnosticsOwnership$", "-test.timeout=40s")
		cmd.Env = append(os.Environ(), marker+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("isolated diagnostics: %v\n%s", err, output)
		}
		return
	}
	config.Cfg = config.Config{PublicOrigin: "https://ican.invalid", DataEncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", StorageDriver: "sqlite", DatabaseDSN: ":memory:", AILogDir: t.TempDir()}
	blockProtocolNetwork(t)
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	connection, _ := db.DB()
	connection.SetMaxOpenConns(1)
	defer connection.Close()
	owner := model.User{ID: "owner", AffCode: "owner-code", Username: "owner", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	other := model.User{ID: "other", AffCode: "other-code", Username: "other", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	for _, user := range []model.User{owner, other} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	channel := model.ModelChannel{ID: "custom", Name: "custom", Protocol: "custom", BaseURL: "https://upstream.invalid/v1", APIKey: "fixture-private-key", Models: []string{"fixture-video"}, Enabled: true, Weight: 1, ParameterTranslation: `{poll:{taskId:"id",GET:{url:"/videos/{taskId}"},status:"state",success:["done"],failure:["failed"]},models:{"fixture-video":{POST:{url:"/videos",body:{model,prompt}},result:"url"}}}`}
	if _, err := repository.SaveSettings(model.Settings{Private: model.PrivateSetting{Channels: []model.ModelChannel{channel}}}, "fixture"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	raw := `{"error":{"message":"permission denied fixture-private-key"},"detail":"` + strings.Repeat("extra ", 1000) + `","url":"https://media.invalid/private?signature=hidden-grant","request_id":"owner-request","trailing":"last diagnostic field"}`
	protocolMockHTTP(t, func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-private-key" {
			t.Fatal("auth changed")
		}
		return &http.Response{StatusCode: 401, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(raw))}, nil
	})
	makeRequest := func(user *model.User, method, path, body string) *http.Request {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Model-Channel-ID", channel.ID)
		if user != nil {
			r = r.WithContext(service.WithUser(r.Context(), model.PublicUser(*user)))
		}
		return r
	}
	assertDetail := func(body string) {
		t.Helper()
		for _, value := range []string{"owner-request", "permission denied", "last diagnostic field", strings.Repeat("extra ", 1000)} {
			if !strings.Contains(body, value) {
				t.Fatalf("missing detail: %s", value[:min(40, len(value))])
			}
		}
		for _, secret := range []string{channel.APIKey, "hidden-grant"} {
			if strings.Contains(body, secret) {
				t.Fatal("credential leaked")
			}
		}
	}
	requestBody := `{"model":"fixture-video","_parameterTranslation":{"kind":"video","variables":{"prompt":"original prompt"}}}`
	writer := httptest.NewRecorder()
	AIVideos(writer, makeRequest(&owner, "POST", "/videos", requestBody))
	assertDetail(writer.Body.String())
	translation, err := service.NewParameterTranslation(channel, "fixture-video", map[string]any{"prompt": "original prompt"}, "video")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := translation.Snapshot(service.ParameterTranslationInput{Variables: map[string]any{"prompt": "original prompt"}}, service.ParameterTranslationResult{TaskIDs: []string{"original-id"}})
	task, err := service.CreateVideoTask(service.VideoTaskCreateInput{UserID: owner.ID, Model: "fixture-video", ChannelID: channel.ID, Status: "processing", ParameterTranslationSnapshot: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	update, err := pollVideoTaskFromUpstream(context.Background(), task)
	if err != nil || update.Status != "failed" {
		t.Fatalf("poll %v %s", err, update.Status)
	}
	if err := service.UpdateVideoTaskFromPoll(task, update); err != nil {
		t.Fatal(err)
	}
	writer = httptest.NewRecorder()
	AIVideo(writer, makeRequest(&owner, "GET", "/videos/"+task.ID+"?model=fixture-video", ""), task.ID)
	var envelope struct{ Data map[string]any }
	if err := json.Unmarshal(writer.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	assertDetail(envelope.Data["error_detail"].(string))
	var diagnostic service.UpstreamError
	if err := json.Unmarshal([]byte(envelope.Data["error_detail"].(string)), &diagnostic); err != nil || diagnostic.Status != 401 || diagnostic.Stage != "轮询" {
		t.Fatal("poll diagnostic context lost", err)
	}
	snapshot = translation.Snapshot(service.ParameterTranslationInput{Variables: map[string]any{"prompt": "original prompt"}}, service.ParameterTranslationResult{ContentURL: channel.BaseURL + "/videos/original-id/content"})
	download, err := service.CreateVideoTask(service.VideoTaskCreateInput{UserID: owner.ID, Model: "fixture-video", ChannelID: channel.ID, Status: "completed", ParameterTranslationSnapshot: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	writer = httptest.NewRecorder()
	AIVideoContent(writer, makeRequest(&owner, "GET", "/videos/"+download.ID+"/content?model=fixture-video", ""), download.ID)
	assertDetail(writer.Body.String())
	if calls != 3 {
		t.Fatalf("unexpected retry: %d", calls)
	}
	for _, user := range []*model.User{nil, &other} {
		for _, content := range []bool{false, true} {
			id := task.ID
			if content {
				id = download.ID
			}
			w := httptest.NewRecorder()
			r := makeRequest(user, "GET", "/videos/"+id+"?model=fixture-video", "")
			if content {
				AIVideoContent(w, r, id)
			} else {
				AIVideo(w, r, id)
			}
			if strings.Contains(w.Body.String(), "owner-request") || strings.Contains(w.Body.String(), "last diagnostic field") || !strings.Contains(w.Body.String(), `"code":1`) {
				t.Fatalf("ownership leak: %s", w.Body)
			}
		}
	}
	if calls != 3 {
		t.Fatalf("foreign task reached upstream: %d", calls)
	}
	logs, err := service.ListAICallLogs(model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(logs)
	for _, value := range []string{"owner-request", "last diagnostic field", channel.APIKey, "hidden-grant"} {
		if strings.Contains(string(encoded), value) {
			t.Fatal("public log leaked", value)
		}
	}
}
