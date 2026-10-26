package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type interruptedVideoReader struct{ io.Reader }

func (r interruptedVideoReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		err = errors.New("connection reset fixture-key https://media.invalid/?signature=secret")
	}
	return n, err
}

func TestVideoHTTPDeadlineAndDisconnectedStream(t *testing.T) {
	video, err := os.ReadFile("testdata/retrieval-video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint("deadline=", deadline), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("Content-Length", fmt.Sprint(len(video)))
				w.Header().Set("X-Request-Id", "stream-request")
				_, _ = w.Write(video[:100])
				w.(http.Flusher).Flush()
				if deadline {
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			client := server.Client()
			if deadline {
				client.Timeout = 50 * time.Millisecond
			}
			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			file, err := VerifyVideoDownload(response)
			if err == nil {
				file.Close()
				t.Fatal("interrupted HTTP accepted")
			}
			var upstream *UpstreamError
			if !errors.As(err, &upstream) || upstream.Stage != "下载" || upstream.RequestID != "stream-request" || !strings.Contains(upstream.Detail, "100 字节") {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifiedVideoDownload(t *testing.T) {
	video, err := os.ReadFile("testdata/retrieval-video.mp4")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		reader io.Reader
		length int64
		want   string
	}{
		{"complete", bytes.NewReader(video), int64(len(video)), ""},
		{"unknown length", bytes.NewReader(video), -1, ""},
		{"short", bytes.NewReader(video[:len(video)/2]), int64(len(video)), "长度不符"},
		{"long", bytes.NewReader(video), int64(len(video) - 1), "长度不符"},
		{"zero declared length", bytes.NewReader(video), 0, "长度不符"},
		{"interrupted", interruptedVideoReader{bytes.NewReader(video[:100])}, int64(len(video)), "下载中断"},
		{"missing index", bytes.NewReader(video[:len(video)/2]), -1, "MP4"},
		{"html", strings.NewReader("<html>private</html>"), -1, "未返回有效视频"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := &http.Response{StatusCode: 200, Header: http.Header{"X-Request-Id": {"fixture-id"}}, ContentLength: tc.length, Body: io.NopCloser(tc.reader)}
			file, err := VerifyVideoDownload(response, "fixture-key")
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				b, _ := io.ReadAll(file.File)
				if !bytes.Equal(b, video) {
					t.Fatal("content changed")
				}
				return
			}
			if err == nil {
				file.Close()
				t.Fatal("corrupt file accepted")
			}
			var upstream *UpstreamError
			if !errors.As(err, &upstream) || !strings.Contains(upstream.Error(), tc.want) || upstream.RequestID != "fixture-id" {
				t.Fatal(err)
			}
			if strings.Contains(upstream.Diagnostics(), "fixture-key") || strings.Contains(upstream.Diagnostics(), "signature=secret") {
				t.Fatal("secret leaked")
			}
		})
	}
	t.Run("slow stream finishes without total-task deadline", func(t *testing.T) {
		r, w := io.Pipe()
		done := make(chan error, 1)
		go func() {
			file, err := VerifyVideoDownload(&http.Response{StatusCode: 200, Header: make(http.Header), Body: r, ContentLength: int64(len(video))})
			if file != nil {
				file.Close()
			}
			done <- err
		}()
		_, _ = w.Write(video[:100])
		select {
		case err := <-done:
			t.Fatalf("premature delivery %v", err)
		case <-time.After(20 * time.Millisecond):
		}
		_, _ = w.Write(video[100:])
		w.Close()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}
