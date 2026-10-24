package service

import (
	"errors"
	"github.com/tigerowo/infinite-canvas/model"
	"strings"
)

func BuildTokenDanceChannelURL(channel model.ModelChannel, path string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(channel.BaseURL), "/")
	lowerPath := strings.ToLower(path)
	for _, prefix := range []string{"/ark/", "/kling/", "/alibaba/", "/minimax/"} {
		if !strings.HasPrefix(lowerPath, prefix) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
			baseURL = strings.TrimRight(baseURL[:len(baseURL)-len("/v1")], "/")
		}
		break
	}
	return baseURL + path
}

func TokenDanceSupportedProtocols(string) ([]string, error) {
	return nil, errors.New("内置远程供应商目录已移除；请明确配置自己的模型")
}

// Retained only to explain errors from historical task records; no external lookup.
func TokenDanceRecoveryMessage(action string) string {
	switch strings.TrimSpace(action) {
	case "top_up_balance":
		return "外部渠道余额不足"
	case "reauthorize_api_key":
		return "外部渠道 API Key 已失效"
	case "api_key_quota":
		return "外部渠道 API Key 已达到额度"
	default:
		return ""
	}
}
