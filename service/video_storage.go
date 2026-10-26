package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/repository"
)

func storeVerifiedVideo(ctx context.Context, id string, video *VerifiedVideo, input *StorageObjectProviderInput) (model.StorageObject, func(), error) {
	settings, err := repository.GetSettings()
	if err != nil {
		return model.StorageObject{}, nil, err
	}
	storage := normalizePrivateStorageSetting(settings.Private.Storage)
	var provider model.StorageProvider
	if input != nil && storage.AllowUserProvider {
		provider = normalizeUserStorageProvider(*input, ctx)
		if !provider.Enabled || !storageProviderConfigured(provider) {
			return model.StorageObject{}, nil, errors.New("用户对象存储配置不完整")
		}
	} else {
		if !canUseGlobalStorage(ctx, storage) {
			return model.StorageObject{}, nil, errors.New("服务端对象存储未启用")
		}
		provider, err = selectStorageProvider(storage)
		if err != nil {
			return model.StorageObject{}, nil, err
		}
	}
	user, _ := UserFromContext(ctx)
	key := strings.Trim(strings.Trim(provider.PathPrefix, "/")+"/"+user.ID+"/video-results/"+id+extensionForContentType(video.MimeType), "/")
	cleanup := func() { _ = deleteStorageObjectData(provider, key) }
	if err = putVerifiedVideo(ctx, provider, key, video); err != nil {
		cleanup()
		return model.StorageObject{}, nil, NewUpstreamError([]byte(err.Error()), 0, "保存", nil, nil, provider.SecretAccessKey, provider.Password)
	}
	object := model.StorageObject{ID: id, ProviderID: provider.ID, Bucket: provider.Bucket, ObjectKey: key, PublicURL: objectURL(provider, key), MimeType: video.MimeType, Bytes: video.Bytes, SHA256: video.SHA256, CreatedBy: user.ID, CreatedAt: now()}
	return object, cleanup, nil
}

func putVerifiedVideo(ctx context.Context, provider model.StorageProvider, key string, video *VerifiedVideo) error {
	reader := io.NewSectionReader(video.File, 0, video.Bytes)
	switch provider.Type {
	case "local":
		if err := video.File.Sync(); err != nil {
			return err
		}
		root, err := localMediaRoot()
		if err != nil {
			return err
		}
		defer root.Close()
		if err = root.MkdirAll(path.Dir(key), 0700); err != nil {
			return err
		}
		return root.Rename(filepath.Base(video.File.Name()), key)
	case model.StorageProviderTypeS3:
		request, err := newS3Request(http.MethodPut, provider, key, reader, video.Bytes)
		if err != nil {
			return err
		}
		request = request.WithContext(ctx)
		request.Header.Set("Content-Type", video.MimeType)
		response, err := SafeProxyHTTPClient().Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("对象存储上传失败：HTTP %d", response.StatusCode)
		}
		return nil
	case model.StorageProviderTypeWebDAV:
		client, err := newWebDAVClient(provider)
		if err != nil {
			return err
		}
		remote, err := cleanStoragePath(key)
		if err != nil {
			return err
		}
		if err = client.MkdirAll(path.Dir(remote), 0755); err != nil {
			return err
		}
		return client.WriteStream(remote, reader, 0644)
	default:
		return errors.New("存储类型不支持")
	}
}
