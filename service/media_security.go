package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

const LocalMediaProvider = "ican-local"

func IsPassiveMedia(contentType string) bool {
	switch strings.Split(contentType, ";")[0] {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif", "image/heif", "image/heic", "image/bmp", "video/mp4", "video/webm", "video/quicktime", "video/ogg", "audio/mpeg", "audio/mp3", "audio/mp4", "audio/aac", "audio/wav", "audio/x-wav", "audio/flac", "audio/x-flac", "audio/ogg", "application/ogg":
		return true
	}
	return false
}

func PassiveMediaType(data []byte) (string, error) {
	contentType := mimetype.Detect(data).String()
	if !IsPassiveMedia(contentType) {
		return "", errors.New("仅支持图片、视频和音频；不接受 HTML、SVG 或未知格式")
	}
	return strings.Split(contentType, ";")[0], nil
}

func localMediaRoot() (*os.Root, error) {
	if config.Cfg.MediaDir == "" {
		return nil, errors.New("未配置 MEDIA_DIR")
	}
	if err := os.MkdirAll(config.Cfg.MediaDir, 0700); err != nil {
		return nil, err
	}
	return os.OpenRoot(config.Cfg.MediaDir)
}

func writeLocalMedia(key string, data []byte) error {
	root, err := localMediaRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(path.Dir(key), 0700); err != nil {
		return err
	}
	f, err := root.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func OpenLocalMedia(key string) (*os.File, error) {
	root, err := localMediaRoot()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(key)
}

func deleteLocalMedia(key string) error {
	root, err := localMediaRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Remove(key)
}

func OwnStorageObject(ctx context.Context, id string) (model.StorageObject, error) {
	object, err := repository.GetStorageObject(id)
	user, ok := UserFromContext(ctx)
	if err != nil || !ok || user.ID == "" || object.CreatedBy != user.ID || object.DeletedAt != "" {
		return model.StorageObject{}, errors.New("文件不存在或无访问权限")
	}
	return object, nil
}

type mediaClaims struct {
	Owner   string `json:"owner"`
	Version uint64 `json:"version"`
	jwt.RegisteredClaims
}

func SignedMediaURL(ctx context.Context, id string) (string, error) {
	object, err := OwnStorageObject(ctx, id)
	if err != nil {
		return "", err
	}
	if config.Cfg.MediaLinkTTLSeconds <= 0 {
		return "", errors.New("管理员尚未确认供应商素材链接有效期")
	}
	claims := mediaClaims{Owner: object.CreatedBy, Version: object.ShareVersion, RegisteredClaims: jwt.RegisteredClaims{Issuer: "ican-media", Subject: id, Audience: jwt.ClaimStrings{"media-read"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.Cfg.MediaLinkTTLSeconds) * time.Second))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(config.Cfg.JWTSecret))
	if err != nil {
		return "", err
	}
	return config.Cfg.PublicOrigin + "/api/files/" + url.PathEscape(id) + "/content?grant=" + url.QueryEscape(token), nil
}

func CanReadStorageObject(r *http.Request, id string) bool {
	if _, err := OwnStorageObject(r.Context(), id); err == nil {
		return true
	}
	claims := mediaClaims{}
	token, err := jwt.ParseWithClaims(r.URL.Query().Get("grant"), &claims, func(t *jwt.Token) (any, error) { return []byte(config.Cfg.JWTSecret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("ican-media"), jwt.WithAudience("media-read"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid || claims.Subject != id {
		return false
	}
	object, err := repository.GetStorageObject(id)
	if err != nil || object.CreatedBy != claims.Owner || object.ShareVersion != claims.Version || object.DeletedAt != "" {
		return false
	}
	owner, ok, err := repository.GetUserByID(claims.Owner)
	return err == nil && ok && owner.Status == model.UserStatusActive
}

// Rewrites only media-address fields, never prompt/model/key strings. A local
// reference must belong to the authenticated caller before granting access.
func AuthorizeMediaReferences(ctx context.Context, data []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return data, nil
	}
	changed := false
	var visit func(any, string) (any, error)
	visit = func(v any, key string) (any, error) {
		switch x := v.(type) {
		case map[string]any:
			for k, item := range x {
				n, e := visit(item, k)
				if e != nil {
					return nil, e
				}
				x[k] = n
			}
		case []any:
			for i, item := range x {
				n, e := visit(item, key)
				if e != nil {
					return nil, e
				}
				x[i] = n
			}
		case string:
			if !mediaAddressField(key) {
				return x, nil
			}
			u, e := url.Parse(x)
			if e != nil {
				return x, nil
			}
			if u.IsAbs() && u.Scheme+"://"+u.Host != config.Cfg.PublicOrigin {
				return x, nil
			}
			if !u.IsAbs() && u.Host != "" {
				return x, nil
			}
			if strings.HasPrefix(u.Path, "/api/files/") && strings.HasSuffix(u.Path, "/content") {
				id := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/api/files/"), "/content")
				if strings.Contains(id, "/") {
					return nil, errors.New("素材地址无效")
				}
				signed, e := SignedMediaURL(ctx, id)
				if e != nil {
					return nil, e
				}
				changed = true
				return signed, nil
			}
		}
		return v, nil
	}
	value, err := visit(value, "")
	if err != nil {
		return nil, err
	}
	if !changed {
		return data, nil
	}
	return json.Marshal(value)
}

func ReadPassivePrefix(body io.ReadCloser) ([]byte, string, error) {
	// Prefix sniffing is not a file-size limit. Stream the remaining body.
	prefix := make([]byte, 512)
	n, err := io.ReadFull(body, prefix)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, "", err
	}
	contentType, e := PassiveMediaType(prefix[:n])
	return prefix[:n], contentType, e
}

func mediaAddressField(key string) bool {
	switch key {
	case "url", "image", "image_url", "video_url", "audio_url", "input_reference[]", "video_reference[]", "audio_reference[]", "first_frame_url", "last_frame_url", "input_reference", "image_urls", "video_urls", "audio_urls":
		return true
	}
	return false
}
func RevokeMediaLinks(ctx context.Context, id string) error {
	if _, err := OwnStorageObject(ctx, id); err != nil {
		return err
	}
	return repository.RevokeMediaLinks(id)
}
