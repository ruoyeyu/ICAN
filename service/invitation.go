package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
	"strings"
	"time"
)

func invitationHash(code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}
func CreateInvitation(adminID string, expiresAt int64) (model.Invitation, string, error) {
	if expiresAt < 0 || (expiresAt > 0 && expiresAt <= time.Now().Unix()) {
		return model.Invitation{}, "", errors.New("过期时间必须在未来")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return model.Invitation{}, "", err
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	item := model.Invitation{ID: newID("invite"), CodeHash: invitationHash(code), Code: code, CreatedBy: adminID, CreatedAt: now(), ExpiresAt: expiresAt}
	return item, code, repository.SaveInvitation(item)
}
