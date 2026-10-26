package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/service"
)

func RetrieveVideoTask(w http.ResponseWriter, r *http.Request, id string) {
	user, ok := service.UserFromContext(r.Context())
	if !ok {
		FailWithStatus(w, 401, "请先登录")
		return
	}
	task, found, err := service.GetUserVideoTask(user.ID, id)
	if err != nil || !found {
		FailWithStatus(w, 404, "视频任务不存在或无权访问")
		return
	}
	var input struct {
		RetryAttempt *uint64                             `json:"retryAttempt"`
		Provider     *service.StorageObjectProviderInput `json:"provider"`
	}
	if err = json.NewDecoder(r.Body).Decode(&input); err != nil && err != io.EOF {
		Fail(w, "取回请求格式错误")
		return
	}
	if _, err = service.StartVideoRetrieval(r.Context(), task, input.RetryAttempt, input.Provider, openVideoTaskContent); err != nil {
		FailError(w, err)
		return
	}
	OK(w, service.VideoTaskResponse(task))
}

func openVideoTaskContent(ctx context.Context, task model.VideoTask) (*http.Response, string, error) {
	var channel model.ModelChannel
	var err error
	user, ok := service.UserFromContext(ctx)
	if !ok || user.ID != task.UserID {
		return nil, "", errors.New("视频任务不存在或无权访问")
	}
	channel, _, err = selectAIRequestChannel(user, task.Model, task.ChannelID, task.UserChannelID, false)
	if err != nil {
		return nil, "", err
	}
	if task.ParameterTranslationSnapshot != "" && task.VideoURL == "" {
		response, err := service.ParameterTranslationVideoContent(ctx, task, channel)
		return response, channel.APIKey, err
	}
	target := task.VideoURL
	protected := service.IsGeminiChannel(channel)
	if strings.HasPrefix(target, "data:video/") {
		prefix, data, ok := strings.Cut(target, ",")
		if !ok || !strings.HasSuffix(prefix, ";base64") {
			return nil, "", errors.New("视频数据编码无效")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), ContentLength: -1, Body: io.NopCloser(base64.NewDecoder(base64.StdEncoding, strings.NewReader(data)))}, "", nil
	}
	if media, parseErr := url.Parse(target); parseErr == nil && strings.Contains(media.Path, "/videos/") && strings.HasSuffix(media.Path, "/content") {
		base, _ := url.Parse(channel.BaseURL)
		if base != nil && media.Scheme == base.Scheme && media.Host == base.Host {
			protected = true
		}
	}
	if target == "" {
		id := firstNonEmpty(task.UpstreamTaskID, task.ID)
		target = resolveAIProxyURL(channel, task.Model, resolveAIProxyPath(channel, task.Model, "/videos/"+url.PathEscape(id)+"/content"))
		protected = true
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, channel.APIKey, err
	}
	if protected {
		service.SetModelChannelAuthHeader(request, channel)
	}
	client := *service.HTTPClientForChannel(channel)
	if protected {
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("鉴权内容下载不允许重定向") }
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, channel.APIKey, service.NewUpstreamError([]byte(err.Error()), 0, "下载", nil, request, channel.APIKey)
	}
	if response.StatusCode != 200 || strings.Contains(response.Header.Get("Content-Type"), "json") || strings.HasPrefix(response.Header.Get("Content-Type"), "text/") {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, service.UpstreamErrorBodyLimit+1))
		return nil, channel.APIKey, service.NewUpstreamError(body, response.StatusCode, "下载", response.Header, request, channel.APIKey)
	}
	return response, channel.APIKey, nil
}
