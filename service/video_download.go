package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	mp4 "github.com/abema/go-mp4"
	"github.com/tigerowo/infinite-canvas/config"
)

type VerifiedVideo struct {
	File             *os.File
	Bytes            int64
	MimeType, SHA256 string
}

func (v *VerifiedVideo) Close() { name := v.File.Name(); v.File.Close(); os.Remove(name) }

// Called once before accepting transfers in the single API process.
func CleanupInterruptedVideoDownloads() error {
	if config.Cfg.MediaDir == "" {
		return nil
	}
	root, err := localMediaRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), ".video-download-") {
			if err := root.Remove(entry.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Stage privately on disk. No success headers or media index exist until EOF,
// length and container checks succeed. Existing channel HTTP deadlines still apply.
func VerifyVideoDownload(response *http.Response, secrets ...string) (_ *VerifiedVideo, err error) {
	failure := func(err error) error {
		return NewUpstreamError([]byte(err.Error()), response.StatusCode, "下载", response.Header, response.Request, secrets...)
	}
	if response.StatusCode != http.StatusOK {
		return nil, failure(fmt.Errorf("视频内容状态异常：HTTP %d", response.StatusCode))
	}
	dir := config.Cfg.MediaDir
	if dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, failure(err)
		}
	}
	f, err := os.CreateTemp(dir, ".video-download-*")
	if err != nil {
		return nil, failure(err)
	}
	v := &VerifiedVideo{File: f}
	defer func() {
		if err != nil {
			v.Close()
		}
	}()
	hash := sha256.New()
	v.Bytes, err = io.Copy(io.MultiWriter(f, hash), response.Body)
	if err != nil {
		return nil, failure(fmt.Errorf("视频下载中断（已接收 %d 字节）：%w", v.Bytes, err))
	}
	expected := response.ContentLength
	if header := response.Header.Get("Content-Length"); header != "" {
		expected, err = strconv.ParseInt(header, 10, 64)
		if err != nil || expected < 0 {
			return nil, failure(errors.New("视频预期长度无效"))
		}
	}
	if expected >= 0 && expected != v.Bytes {
		return nil, failure(fmt.Errorf("视频长度不符：预期 %d 字节，收到 %d 字节", expected, v.Bytes))
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, failure(err)
	}
	_, v.MimeType, err = ReadPassivePrefix(f)
	if err != nil || !strings.HasPrefix(v.MimeType, "video/") {
		return nil, failure(errors.New("视频内容接口未返回有效视频"))
	}
	if v.MimeType == "video/mp4" || v.MimeType == "video/quicktime" {
		if err = validateMP4Container(f, v.Bytes); err != nil {
			return nil, failure(err)
		}
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, failure(err)
	}
	v.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return v, nil
}

func validateMP4Container(file *os.File, size int64) error {
	var moov, mdat bool
	for offset := uint64(0); offset < uint64(size); {
		if _, err := file.Seek(int64(offset), io.SeekStart); err != nil {
			return err
		}
		box, err := mp4.ReadBoxInfo(file)
		if err != nil || box.Size < box.HeaderSize || box.Size > uint64(size)-offset {
			return errors.New("视频 MP4 容器不完整")
		}
		if box.Type == mp4.BoxTypeMoov() {
			moov = box.Size > box.HeaderSize
		}
		if box.Type == mp4.BoxTypeMdat() {
			mdat = box.Size > box.HeaderSize
		}
		offset += box.Size
	}
	if !moov || !mdat {
		return errors.New("视频 MP4 缺少 moov 索引或媒体数据")
	}
	return nil
}
