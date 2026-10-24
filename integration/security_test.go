package integration_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
	"github.com/tigerowo/infinite-canvas/router"
	"github.com/tigerowo/infinite-canvas/service"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSecurityBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config.Cfg = config.Config{PublicOrigin: "https://ican.invalid", DataEncryptionKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), JWTSecret: strings.Repeat("test-only-secret", 4), JWTExpireHours: 1, StorageDriver: "sqlite", DatabaseDSN: ":memory:", MediaDir: t.TempDir(), AILogDir: t.TempDir(), MediaLinkTTLSeconds: 60, AuthAttemptsPerMinute: 1000}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	r := router.New()
	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	successful := func(w *httptest.ResponseRecorder) bool {
		var v struct {
			Code int `json:"code"`
		}
		return w.Code == 200 && json.Unmarshal(w.Body.Bytes(), &v) == nil && v.Code == 0
	}
	t.Run("default-admin-refused", func(t *testing.T) {
		if service.EnsureDefaultAdmin() == nil {
			t.Fatal("bootstrapped missing administrator secret")
		}
	})
	admin, err := service.SaveUser(model.User{Username: "admin-test", Role: model.UserRoleAdmin}, "fixture-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	adminSession, err := service.Login(admin.Username, "fixture-admin-password")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("invite-required", func(t *testing.T) {
		if successful(request("POST", "/api/auth/register", "", `{"username":"outsider","password":"long-enough-fixture"}`)) {
			t.Fatal("registered without invitation")
		}
	})
	_, code, err := service.CreateInvitation(admin.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := service.Register("alice", "alice-password", code)
	if err != nil {
		t.Fatal(err)
	}
	_, code2, _ := service.CreateInvitation(admin.ID, 0)
	bob, err := service.Register("bob", "bob-password", code2)
	if err != nil {
		t.Fatal(err)
	}
	aliceCtx := service.WithUser(context.Background(), alice.User)
	t.Run("invite-single-use-and-hashed", func(t *testing.T) {
		if _, err := service.Register("third", "password", code); err == nil {
			t.Fatal("reused invitation")
		}
		var items []model.Invitation
		db.Find(&items)
		for _, item := range items {
			if item.CodeHash == code || len(item.CodeHash) != 64 {
				t.Fatal("invitation not hashed")
			}
		}
		revoked, c, _ := service.CreateInvitation(admin.ID, 0)
		repository.RevokeInvitation(revoked.ID)
		if _, err := service.Register("fourth", "password", c); err == nil {
			t.Fatal("revoked invitation accepted")
		}
		exp, c, _ := service.CreateInvitation(admin.ID, time.Now().Add(time.Hour).Unix())
		db.Model(&exp).Update("expires_at", time.Now().Add(-time.Second).Unix())
		if _, err := service.Register("fifth", "password", c); err == nil {
			t.Fatal("expired invitation accepted")
		}
	})
	t.Run("roles-and-disabled-surfaces", func(t *testing.T) {
		for _, token := range []string{"", alice.Token} {
			if successful(request("GET", "/api/admin/invitations", token, "")) {
				t.Fatal("admin boundary failed")
			}
		}
		if !successful(request("GET", "/api/admin/invitations", adminSession.Token, "")) {
			t.Fatal("admin denied")
		}
		for _, path := range []string{"/api/bridge/comfy/poll", "/api/v1/workflows", "/api/auth/linux-do/authorize", "/api/agent-skills"} {
			if got := request("GET", path, adminSession.Token, ""); got.Code != 404 {
				t.Fatalf("disabled %s: %d", path, got.Code)
			}
		}
		if request("GET", "/api/proxy-image?url=https://example.com/a", "", "").Code != 401 {
			t.Fatal("anonymous proxy open")
		}
	})
	t.Run("config-encryption-and-two-user-recovery", func(t *testing.T) {
		raw := `{"localChannels":[{"id":"private","protocol":"custom","baseUrl":"https://provider.invalid","apiKey":"fixture-secret-key","models":["my-video"]}],"videoModel":"my-video"}`
		if _, err := service.SaveCurrentUserModelConfig(aliceCtx, json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
		var row model.UserConfig
		db.First(&row, "user_id = ?", alice.User.ID)
		if strings.Contains(row.ModelConfig, "fixture-secret-key") || !strings.HasPrefix(row.ModelConfig, "ican:v1:") {
			t.Fatal("plaintext persisted")
		}
		fresh, err := service.Login("alice", "alice-password")
		if err != nil {
			t.Fatal(err)
		}
		if w := request("GET", "/api/v1/user-config", fresh.Token, ""); !successful(w) || !strings.Contains(w.Body.String(), "fixture-secret-key") {
			t.Fatal("cross-device recovery failed")
		}
		if w := request("GET", "/api/v1/user-config", bob.Token, ""); strings.Contains(w.Body.String(), "fixture-secret-key") {
			t.Fatal("cross-user key disclosure")
		}
		settings, _ := repository.GetSettings()
		off := false
		settings.Public.ModelChannel.AllowCustomChannel = &off
		repository.SaveSettings(settings, "fixture")
		if _, err := service.SelectUserLocalModelChannelForModel(alice.User.ID, "my-video", "private"); err == nil {
			t.Fatal("disabled custom channel usable")
		}
		if w := request("POST", "/api/ai/direct-request", alice.Token, `{"channel":{"protocol":"kie","baseUrl":"https://supplier.invalid"},"model":"fixture","endpoint":"/videos","body":{}}`); !strings.Contains(w.Body.String(), "管理员已关闭自定义渠道") {
			t.Fatal("direct translation bypassed disabled custom channels")
		}
		on := true
		settings.Public.ModelChannel.AllowCustomChannel = &on
		repository.SaveSettings(settings, "fixture")
	})
	t.Run("admin-cannot-enable-removed-protocols", func(t *testing.T) {
		for _, protocol := range []string{"tokendance", "autodl", "runninghub", "comfyui"} {
			channel := model.ModelChannel{Protocol: protocol, BaseURL: "https://supplier.invalid", APIKey: "fixture-key"}
			if _, err := service.AdminChannelModels(nil, channel); err == nil || !strings.Contains(err.Error(), "未启用") {
				t.Fatalf("admin discovery accepted disabled protocol %s: %v", protocol, err)
			}
			settings, _ := repository.GetSettings()
			settings.Private.Channels = []model.ModelChannel{channel}
			if _, err := service.SaveSettings(settings); err == nil {
				t.Fatalf("admin saved disabled protocol %s", protocol)
			}
		}
	})
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	fileID := "private-media"
	if err := db.Create(&model.StorageObject{ID: fileID, ProviderID: service.LocalMediaProvider, ObjectKey: "picture.png", CreatedBy: alice.User.ID, MimeType: "image/png", Bytes: int64(buf.Len())}).Error; err != nil {
		t.Fatal(err)
	}
	// Write a fixture only; production upload uses os.Root and content sniffing.
	root, err := os.OpenRoot(config.Cfg.MediaDir)
	if err != nil {
		t.Fatal(err)
	}
	if err = root.WriteFile("picture.png", buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	root.Close()
	t.Run("private-media-share-expiry-revocation-range", func(t *testing.T) {
		for _, token := range []string{"", bob.Token} {
			if request("GET", "/api/files/"+fileID+"/content", token, "").Code != 404 {
				t.Fatal("private media leaked")
			}
		}
		w := request("GET", "/api/files/"+fileID+"/content", alice.Token, "")
		if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), buf.Bytes()) {
			t.Fatalf("owner download failed %d", w.Code)
		}
		req := httptest.NewRequest("GET", "/api/files/"+fileID+"/content", nil)
		req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: alice.Token})
		req.Header.Set("Range", "bytes=0-7")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 206 || w.Body.Len() != 8 {
			t.Fatal("authenticated range/cookie failed")
		}
		link, err := service.SignedMediaURL(aliceCtx, fileID)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(link)
		if request("GET", u.RequestURI(), "", "").Code != 200 {
			t.Fatal("authorized reference failed")
		}
		if request("GET", strings.Replace(u.RequestURI(), fileID, "other", 1), "", "").Code != 404 {
			t.Fatal("grant accepted for another object")
		}
		if _, err := service.SignedMediaURL(service.WithUser(context.Background(), bob.User), fileID); err == nil {
			t.Fatal("cross-user signing")
		}
		if err := service.RevokeMediaLinks(aliceCtx, fileID); err != nil {
			t.Fatal(err)
		}
		if request("GET", u.RequestURI(), "", "").Code != 404 {
			t.Fatal("revoked grant accepted")
		}
		config.Cfg.MediaLinkTTLSeconds = -1
		if _, err := service.SignedMediaURL(aliceCtx, fileID); err == nil {
			t.Fatal("unconfigured TTL accepted")
		}
		config.Cfg.MediaLinkTTLSeconds = 60
		if successful(request("DELETE", "/api/v1/files/"+fileID, bob.Token, "")) {
			t.Fatal("cross-user delete")
		}
	})

	t.Run("canvas-assets-and-task-owner-boundaries", func(t *testing.T) {
		project := `{"id":"same-project","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z","title":"alice-private"}`
		if _, err := service.SaveCurrentUserCanvasProject(aliceCtx, json.RawMessage(project)); err != nil {
			t.Fatal(err)
		}
		if w := request("GET", "/api/v1/canvas/projects", bob.Token, ""); strings.Contains(w.Body.String(), "alice-private") {
			t.Fatal("cross-user canvas disclosed")
		}
		request("POST", "/api/v1/canvas/projects/delete", bob.Token, `{"ids":["same-project"]}`)
		if w := request("GET", "/api/v1/canvas/projects", alice.Token, ""); !strings.Contains(w.Body.String(), "alice-private") {
			t.Fatal("cross-user canvas deleted")
		}
		request("POST", "/api/v1/user-data/assets", alice.Token, `{"data":[{"name":"private-asset"}]}`)
		if w := request("GET", "/api/v1/user-data/assets", bob.Token, ""); strings.Contains(w.Body.String(), "private-asset") {
			t.Fatal("cross-user assets disclosed")
		}
		if _, err := repository.SaveCanvasImageTask(model.CanvasImageTask{ID: "collision-image", UserID: alice.User.ID, Prompt: "private-prompt"}); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.SaveCanvasImageTask(model.CanvasImageTask{ID: "collision-image", UserID: bob.User.ID, Prompt: "overwrite"}); err == nil {
			t.Fatal("cross-user image task overwritten")
		}
		if _, found, _ := repository.GetUserCanvasImageTask(bob.User.ID, "collision-image"); found {
			t.Fatal("cross-user image task read")
		}
		if _, err := repository.SaveCanvasAudioTask(model.CanvasAudioTask{ID: "collision-audio", UserID: alice.User.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.SaveCanvasAudioTask(model.CanvasAudioTask{ID: "collision-audio", UserID: bob.User.ID}); err == nil {
			t.Fatal("cross-user audio task overwritten")
		}
		if _, err := repository.SaveVideoTask(model.VideoTask{ID: "collision-video", UserID: alice.User.ID, Status: "processing"}); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.SaveVideoTask(model.VideoTask{ID: "collision-video", UserID: bob.User.ID, Status: "completed"}); err == nil {
			t.Fatal("cross-user video task overwritten")
		}
		if task, found, _ := repository.GetUserVideoTask(alice.User.ID, "collision-video"); !found || task.Status != "processing" {
			t.Fatal("owner video task corrupted")
		}
	})
	t.Run("active-content-rejected", func(t *testing.T) {
		for _, data := range []string{"<!doctype html><script>alert(1)</script>", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`} {
			if _, err := service.PassiveMediaType([]byte(data)); err == nil {
				t.Fatal("active content accepted")
			}
		}
		old := service.SafeProxyHTTPClient().Transport
		defer func() { service.SafeProxyHTTPClient().Transport = old }()
		service.SafeProxyHTTPClient().Transport = transport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"image/png"}}, Body: io.NopCloser(strings.NewReader("<!doctype html><html>active</html>"))}, nil
		})
		if w := request("GET", "/api/proxy-image?url=https://upstream.invalid/fake.png", alice.Token, ""); strings.Contains(w.Body.String(), "<html>") {
			t.Fatal("proxy trusted declared content-type")
		}
	})
	t.Run("csrf", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: service.SessionCookieName, Value: alice.Token})
		req.Header.Set("Origin", "https://attacker.invalid")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal("cross-origin state change accepted")
		}
	})
	t.Run("logout-password-disable-revoke", func(t *testing.T) {
		if !successful(request("POST", "/api/auth/logout", alice.Token, "{}")) {
			t.Fatal("logout failed")
		}
		if _, ok := service.CurrentAuthUser(alice.Token); ok {
			t.Fatal("logged-out token accepted")
		}
		fresh, _ := service.Login("alice", "alice-password")
		if !successful(request("POST", "/api/auth/password", fresh.Token, `{"currentPassword":"alice-password","password":"new-password"}`)) {
			t.Fatal("password change failed")
		}
		if _, ok := service.CurrentAuthUser(fresh.Token); ok {
			t.Fatal("old token survived password change")
		}
		fresh, err := service.Login("alice", "new-password")
		if err != nil {
			t.Fatal(err)
		}
		user, _, _ := repository.GetUserByID(alice.User.ID)
		user.Status = model.UserStatusBan
		if _, err := service.SaveUser(user, ""); err != nil {
			t.Fatal(err)
		}
		if _, ok := service.CurrentAuthUser(fresh.Token); ok {
			t.Fatal("disabled session accepted")
		}
		user.Status = model.UserStatusActive
		service.SaveUser(user, "")
		if _, ok := service.CurrentAuthUser(fresh.Token); ok {
			t.Fatal("re-enabled old session accepted")
		}
	})
	t.Run("login-rate-limit", func(t *testing.T) {
		config.Cfg.AuthAttemptsPerMinute = 1
		freshRouter := router.New()
		for i := 0; i < 2; i++ {
			w := httptest.NewRecorder()
			freshRouter.ServeHTTP(w, httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`)))
			if i == 1 && w.Code != 429 {
				t.Fatal("auth admission limit missing")
			}
		}
	})
}
