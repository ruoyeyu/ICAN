package service

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"

	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/repository"
)

const SessionCookieName = "ican_session"

func RequestToken(r *http.Request) string {
	if value := r.Header.Get("Authorization"); strings.HasPrefix(value, "Bearer ") {
		return strings.TrimPrefix(value, "Bearer ")
	}
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

func WriteSessionCookie(w http.ResponseWriter, token string) {
	maxAge := -1
	if claims, err := ParseToken(token); err == nil {
		maxAge = int(claims.ExpiresAt.Unix() - claims.IssuedAt.Unix())
	}
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: token, Path: "/api", MaxAge: maxAge, HttpOnly: true, Secure: strings.HasPrefix(config.Cfg.PublicOrigin, "https://"), SameSite: http.SameSiteStrictMode})
}

func RevokeSession(token string) error {
	claims, err := ParseToken(token)
	if err != nil {
		return err
	}
	return repository.DeleteLoginSession(claims.ID)
}

func ChangeOwnPassword(ctx context.Context, current, next string) error {
	auth, ok := UserFromContext(ctx)
	if !ok {
		return errors.New("请先登录")
	}
	user, ok, err := repository.GetUserByID(auth.ID)
	if err != nil || !ok {
		return errors.New("用户不存在")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(current)) != nil {
		return errors.New("当前密码不正确")
	}
	if next == "" {
		return errors.New("新密码不能为空")
	}
	_, err = SaveUser(user, next)
	return err
}
