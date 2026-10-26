package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
	"github.com/tigerowo/infinite-canvas/service"
)

func TestVideoRetrievalLifecycle(t *testing.T) {
	const marker = "VIDEO_RETRIEVAL_CHILD"
	if os.Getenv(marker) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestVideoRetrievalLifecycle$", "-test.timeout=40s")
		cmd.Env = append(os.Environ(), marker+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, output)
		}
		return
	}
	config.Cfg = config.Config{PublicOrigin: "https://ican.invalid", MediaDir: t.TempDir(), DataEncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", StorageDriver: "sqlite", DatabaseDSN: ":memory:", AILogDir: t.TempDir()}
	blockProtocolNetwork(t)
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	defer conn.Close()
	owner := model.User{ID: "owner", Username: "owner", AffCode: "owner", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	other := model.User{ID: "other", Username: "other", AffCode: "other", Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	for _, u := range []model.User{owner, other} {
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
	}
	channel := model.ModelChannel{ID: "fixture", Protocol: "custom", BaseURL: "https://provider.invalid/v1", APIKey: "fixture-key", Enabled: true, Weight: 1, Models: []string{"fixture-video"}, ParameterTranslation: `{models:{"fixture-video":{POST:{url:"/videos",body:{model,prompt}},result:"url"}}}`}
	_, err = repository.SaveSettings(model.Settings{Private: model.PrivateSetting{Channels: []model.ModelChannel{channel}}}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	translation, err := service.NewParameterTranslation(channel, "fixture-video", map[string]any{"prompt": "unchanged"}, "video")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := translation.Snapshot(service.ParameterTranslationInput{Variables: map[string]any{"prompt": "unchanged"}}, service.ParameterTranslationResult{Status: "completed", ContentURL: channel.BaseURL + "/videos/original-id/content"})
	task, err := service.CreateVideoTask(service.VideoTaskCreateInput{ClientTaskID: "client_video_task_fixture", UpstreamTaskID: "original-id", UserID: owner.ID, Model: "fixture-video", ChannelID: channel.ID, Status: "completed", ParameterTranslationSnapshot: snapshot})
	if err != nil {
		t.Fatal(err)
	}
	video, err := os.ReadFile("../service/testdata/retrieval-video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	var downloads, creates atomic.Int32
	var broken atomic.Bool
	broken.Store(true)
	gate := make(chan struct{})
	protocolMockHTTP(t, func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" {
			creates.Add(1)
			return nil, io.ErrUnexpectedEOF
		}
		if r.URL.Path != "/v1/videos/original-id/content" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			return nil, io.ErrUnexpectedEOF
		}
		downloads.Add(1)
		<-gate
		body := video
		if broken.Load() {
			body = body[:len(body)/2]
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"video/mp4"}, "X-Request-Id": {"retrieval-request"}}, ContentLength: int64(len(video)), Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
	})
	request := func(user model.User, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/video-tasks/fixture/retrieve", strings.NewReader(body))
		r = r.WithContext(service.WithUser(r.Context(), model.PublicUser(user)))
		w := httptest.NewRecorder()
		RetrieveVideoTask(w, r, task.ID)
		return w
	}
	w := request(other, `{}`)
	if w.Code != 404 {
		t.Fatal("cross-user retrieval allowed")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); request(owner, `{}`) }()
	}
	wg.Wait()
	deadline := time.Now().Add(time.Second)
	for downloads.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if downloads.Load() != 1 {
		t.Fatalf("duplicate download: %d", downloads.Load())
	}
	var count int64
	db.Model(&model.StorageObject{}).Count(&count)
	if count != 0 {
		t.Fatal("partial material published")
	}
	close(gate)
	wait := func(status string) model.VideoRetrieval {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			result, err := repository.GetVideoRetrieval(owner.ID, task.ID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status == status {
				return result
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("state never became %s", status)
		return model.VideoRetrieval{}
	}
	failed := wait("failed")
	if !strings.Contains(failed.ErrorDetail, "长度不符") || !strings.Contains(failed.ErrorDetail, "retrieval-request") {
		t.Fatal(failed.ErrorDetail)
	}
	db.Model(&model.StorageObject{}).Count(&count)
	if count != 0 {
		t.Fatal("truncated material saved")
	}
	for i := 0; i < 4; i++ {
		request(owner, `{}`)
	}
	if downloads.Load() != 1 {
		t.Fatal("automatic retry")
	}
	broken.Store(false)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); request(owner, `{"retryAttempt":1}`) }()
	}
	wg.Wait()
	ready := wait("ready")
	if ready.Attempt != 2 || downloads.Load() != 2 || creates.Load() != 0 {
		t.Fatal("duplicate retry or paid create")
	}
	for i := 0; i < 4; i++ {
		request(owner, `{}`)
		request(owner, `{"retryAttempt":1}`)
	}
	if downloads.Load() != 2 {
		t.Fatal("refresh downloaded again")
	}
	db.Model(&model.StorageObject{}).Count(&count)
	if count != 1 {
		t.Fatalf("saved %d objects", count)
	}
	payload := service.VideoTaskResponse(task)
	if payload["storageKey"] != "server:"+ready.ObjectID || payload["task_id"] != "original-id" || payload["status"] != "completed" {
		t.Fatal("task result not retained")
	}
	object, err := repository.GetStorageObject(ready.ObjectID)
	if err != nil {
		t.Fatal(err)
	}
	f, err := service.OpenLocalMedia(object.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := io.ReadAll(f)
	f.Close()
	if !bytes.Equal(saved, video) {
		t.Fatal("stored bytes differ")
	}
	if err := repository.DeleteFinishedVideoTasksBefore(time.Now().Add(time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := repository.GetUserVideoTask(owner.ID, task.ID); !found {
		t.Fatal("completion deleted before restoration")
	}
	// An old poll must not overwrite the separately persisted delivery record.
	if err := service.UpdateVideoTaskFromPoll(task, service.VideoTaskPollUpdate{Status: "processing", Progress: 30}); err != nil {
		t.Fatal(err)
	}
	if latest, _ := repository.GetVideoRetrieval(owner.ID, task.ID); latest.Status != "ready" {
		t.Fatal("old poll overwrote result")
	}
	if latest, _, _ := repository.GetUserVideoTask(owner.ID, task.ID); latest.Status != "completed" {
		t.Fatal("old poll regressed completed generation")
	}
	encoded, _ := json.Marshal(payload)
	if bytes.Contains(encoded, []byte("fixture-key")) {
		t.Fatal("key exposed")
	}
}
