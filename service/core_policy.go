package service

import (
	"strings"

	"github.com/tigerowo/infinite-canvas/repository"
)

func RequireCustomChannels() error {
	settings, err := repository.GetSettings()
	if err != nil {
		return err
	}
	if allowed := settings.Public.ModelChannel.AllowCustomChannel; allowed != nil && !*allowed {
		return safeMessageError{message: "管理员已关闭自定义渠道"}
	}
	return nil
}

func RequireCoreProtocol(protocol string) error {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "comfyui", "runninghub", "autodl", "tokendance":
		return safeMessageError{message: "当前部署未启用该渠道协议"}
	}
	return nil
}
