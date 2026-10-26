package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tigerowo/infinite-canvas/repository"
	"github.com/tigerowo/infinite-canvas/service"
)

func Logout(w http.ResponseWriter, r *http.Request) {
	if err := service.RevokeSession(service.RequestToken(r)); err != nil {
		FailError(w, err)
		return
	}
	service.WriteSessionCookie(w, "")
	OK(w, true)
}

func AdminInvitations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		items, err := repository.ListInvitations()
		if err != nil {
			FailError(w, err)
			return
		}
		OK(w, items)
		return
	}
	var input struct {
		ExpiresAt int64 `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Fail(w, "邀请码参数格式错误")
		return
	}
	user, _ := service.UserFromContext(r.Context())
	item, code, err := service.CreateInvitation(user.ID, input.ExpiresAt)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, map[string]any{"invitation": item, "code": code})
}

func AdminRevokeInvitation(w http.ResponseWriter, r *http.Request, id string) {
	if err := repository.RevokeInvitation(id); err != nil {
		FailError(w, err)
		return
	}
	OK(w, true)
}

func ShareMedia(w http.ResponseWriter, r *http.Request, id string) {
	link, err := service.SignedMediaURL(r.Context(), id)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, map[string]string{"url": link})
}

func RevokeMediaLinks(w http.ResponseWriter, r *http.Request, id string) {
	if err := service.RevokeMediaLinks(r.Context(), id); err != nil {
		FailError(w, err)
		return
	}
	OK(w, true)
}
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"currentPassword"`
		Password        string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Fail(w, "密码参数无效")
		return
	}
	if err := service.ChangeOwnPassword(r.Context(), input.CurrentPassword, input.Password); err != nil {
		FailError(w, err)
		return
	}
	service.WriteSessionCookie(w, "")
	OK(w, true)
}
