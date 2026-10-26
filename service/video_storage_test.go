package service

import (
	"bytes"
	"context"
	"github.com/tigerowo/infinite-canvas/model"
	"io"
	"net/http"
	"os"
	"testing"
)

type videoStorageTransport func(*http.Request) (*http.Response, error)

func (f videoStorageTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerifiedVideoUsesExistingStorageStreams(t *testing.T) {
	payload, err := os.ReadFile("testdata/retrieval-video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	previous := SafeProxyHTTPClient().Transport
	defer func() { SafeProxyHTTPClient().Transport = previous }()
	for _, kind := range []string{model.StorageProviderTypeS3, model.StorageProviderTypeWebDAV} {
		t.Run(string(kind), func(t *testing.T) {
			puts := 0
			SafeProxyHTTPClient().Transport = videoStorageTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method == "PUT" {
					puts++
					data, err := io.ReadAll(r.Body)
					if err != nil || !bytes.Equal(data, payload) {
						t.Fatal("upload stream corrupted", err)
					}
				}
				return &http.Response{StatusCode: 201, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(nil)), Request: r}, nil
			})
			file, err := VerifyVideoDownload(&http.Response{StatusCode: 200, ContentLength: int64(len(payload)), Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(payload))})
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			provider := model.StorageProvider{Type: kind, Endpoint: "https://storage.invalid", Bucket: "fixture", Region: "auto", AccessKeyID: "fixture-access", SecretAccessKey: "fixture-secret", Username: "fixture-user", Password: "fixture-password"}
			if err := putVerifiedVideo(context.Background(), provider, "owner/video-results/fixture.mp4", file); err != nil {
				t.Fatal(err)
			}
			if puts != 1 {
				t.Fatalf("duplicate storage writes: %d", puts)
			}
		})
	}
}
