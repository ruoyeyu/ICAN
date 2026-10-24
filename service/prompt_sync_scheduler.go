package service

import (
	"errors"
	"github.com/tigerowo/infinite-canvas/model"
)

// Remote prompt subscriptions are deliberately unavailable in ICAN.
func SyncPromptCategory(string) ([]model.PromptCategory, error) {
	return nil, errors.New("远程提示词订阅已移除")
}
func SyncRemotePromptCategories() {}
func normalizePromptSyncSetting(setting model.PromptSyncSetting) model.PromptSyncSetting {
	disabled := false
	setting.Enabled = &disabled
	return setting
}
