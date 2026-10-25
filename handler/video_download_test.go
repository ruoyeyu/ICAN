package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVideoDownloadSniffsActualContent(t *testing.T) {
	mp4 := append([]byte{0, 0, 0, 24}, []byte("ftypisom\x00\x00\x00\x00isommp42")...)
	mp4 = append(mp4, bytes.Repeat([]byte{0}, 600)...)
	for _, payload := range [][]byte{mp4, []byte("<html><script>alert(1)</script></html>"), []byte(`{"error":"denied"}`), nil} {
		w := httptest.NewRecorder()
		r := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(bytes.NewReader(payload))}
		writeVideoDownload(w, r)
		if bytes.Equal(payload, mp4) {
			if w.Header().Get("Content-Type") != "video/mp4" || !bytes.Equal(w.Body.Bytes(), mp4) || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("video stream changed")
			}
		} else if strings.HasPrefix(w.Header().Get("Content-Type"), "video/") || !strings.Contains(w.Body.String(), "未返回有效视频") {
			t.Fatal("active or empty upstream content accepted")
		}
	}
}
