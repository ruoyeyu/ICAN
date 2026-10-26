package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

type VideoContentOpener func(context.Context, model.VideoTask) (*http.Response, string, error)

func StartVideoRetrieval(ctx context.Context, task model.VideoTask, retryAttempt *uint64, provider *StorageObjectProviderInput, open VideoContentOpener) (model.VideoRetrieval, error) {
	user, ok := UserFromContext(ctx)
	if !ok || user.ID != task.UserID {
		return model.VideoRetrieval{}, errors.New("视频任务不存在或无权访问")
	}
	if !IsCompletedVideoTaskStatus(task.Status) {
		return model.VideoRetrieval{}, errors.New("视频尚未生成完成")
	}
	result, claimed, err := repository.ClaimVideoRetrieval(user.ID, task.ID, retryAttempt)
	if err != nil || !claimed {
		return result, err
	}
	// The transfer belongs to the persisted task, not the browser connection.
	go func() {
		ctx := WithUser(context.Background(), user)
		fail := func(err error) {
			var upstream *UpstreamError
			if !errors.As(err, &upstream) {
				upstream = NewUpstreamError([]byte(err.Error()), 0, "保存", nil, nil)
			}
			_ = repository.UpdateVideoRetrieval(result, "failed", upstream.Diagnostics())
		}
		response, key, err := open(ctx, task)
		if err != nil {
			fail(err)
			return
		}
		defer response.Body.Close()
		video, err := VerifyVideoDownload(response, key)
		if err != nil {
			fail(err)
			return
		}
		defer video.Close()
		if err = repository.UpdateVideoRetrieval(result, "saving", ""); err != nil {
			fail(err)
			return
		}
		object, cleanup, err := storeVerifiedVideo(ctx, result.ObjectID, video, provider)
		if err != nil {
			fail(err)
			return
		}
		if err = repository.CompleteVideoRetrieval(result, object); err != nil {
			cleanup()
			fail(err)
		}
	}()
	return result, nil
}

func addVideoRetrievalResponse(task model.VideoTask, response map[string]any) {
	result, err := repository.GetVideoRetrieval(task.UserID, task.ID)
	if err != nil {
		response["retrieval"] = map[string]any{"status": "failed", "errorDetail": "读取视频取回记录失败"}
		return
	}
	if result.TaskID == "" {
		return
	}
	response["retrieval"] = result
	// Upstream completion remains visible, but unverified URLs are not delivered.
	delete(response, "url")
	delete(response, "video_url")
	delete(response, "data")
	if result.Status != "ready" {
		return
	}
	object, err := repository.GetStorageObject(result.ObjectID)
	if err != nil || object.CreatedBy != task.UserID || strings.TrimSpace(object.DeletedAt) != "" {
		_ = repository.InvalidateMissingVideoResult(result)
		response["retrieval"] = map[string]any{"status": "failed", "attempt": result.Attempt, "errorDetail": "已保存的视频不存在"}
		return
	}
	response["url"] = "/api/files/" + object.ID + "/content"
	response["video_url"] = response["url"]
	response["storageKey"] = "server:" + object.ID
	response["bytes"] = object.Bytes
	response["mimeType"] = object.MimeType
}
