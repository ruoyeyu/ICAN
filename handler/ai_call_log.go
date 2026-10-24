package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/tigerowo/infinite-canvas/service"
)

func AdminAICallLogs(w http.ResponseWriter, r *http.Request) {
	list, err := service.ListAICallLogs(parseQuery(r))
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, list)
}

// ClientAICallLog 接收前端本地直连渠道的 AI 调用日志上报。
func ClientAICallLog(w http.ResponseWriter, r *http.Request) {
	user, ok := service.UserFromContext(r.Context())
	if !ok || user.ID == "" {
		Fail(w, "请先登录")
		return
	}
	var request service.AICallLogInput
	_ = json.NewDecoder(r.Body).Decode(&request)
	if !service.LocalDirectAILogEnabled() {
		OK(w, true)
		return
	}
	request.UserID = user.ID
	request.UserDisplayName = firstNonEmpty(user.DisplayName, user.Username)
	service.SaveAICallLog(request)
	OK(w, true)
}

func AdminDeleteAICallLogs(w http.ResponseWriter, r *http.Request) {
	days := 7
	if v := r.URL.Query().Get("olderThanDays"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			days = parsed
		}
	}
	removed, err := service.DeleteAICallLogsOlderThan(days)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, map[string]int{"removedFiles": removed})
}
