package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/tigerowo/infinite-canvas/model"
)

var templateFieldPath = regexp.MustCompile(`^[\w-]+(?:\.[\w-]+)*$`)
var templateHeader = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
var templateCredential = regexp.MustCompile(`(?i)^(api_?key|authorization|password|secret|access_?token|cookie)$`)
var errTemplate = errors.New("模板格式无效：请检查必填字段、协议版本和凭据；不支持可执行代码")

// Validate the existing portable authoring format without executing its contents.
func ParseChannelTemplate(raw []byte) (model.ChannelTemplate, error) {
	var out model.ChannelTemplate
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out) != nil || decoder.Decode(new(any)) != io.EOF {
		return out, errTemplate
	}
	var value any
	if json.Unmarshal(raw, &value) != nil || !templateRequired(value, reflect.TypeOf(out)) || !templateNoCredentials(value) {
		return out, errTemplate
	}
	if out.Format != "ican-channel" || out.Version != 1 || strings.TrimSpace(out.Name) == "" || !templateBaseURL(out.BaseURL) || !slices.Contains([]string{"custom", "openai"}, out.Protocol) || len(out.Models) == 0 || !templateUnique(out.Models) || out.Authoring.Version != 1 {
		return out, errTemplate
	}
	for name, kind := range out.ModelCapabilities {
		if !slices.Contains(out.Models, name) || !slices.Contains([]string{"image", "video", "audio", "text"}, kind) {
			return out, errTemplate
		}
	}
	seen := map[string]bool{}
	for i, m := range out.Authoring.Models {
		if seen[m.Name] || !slices.Contains(out.Models, m.Name) || !validTemplateModel(m) {
			return out, fmt.Errorf("模型配置 #%d 无效：检查字段、鉴权、任务查询与参数声明", i+1)
		}
		seen[m.Name] = true
	}
	return out, nil
}

// encoding/json checks types/unknown fields; this rejects missing/null required
// fields, including booleans whose zero values must not silently fill omissions.
func templateRequired(value any, typ reflect.Type) bool {
	if value == nil {
		return false
	}
	if typ == reflect.TypeOf(json.RawMessage{}) {
		return true
	}
	switch typ.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return false
		}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			tag := strings.Split(field.Tag.Get("json"), ",")
			v, found := object[tag[0]]
			if !found && slices.Contains(tag, "omitempty") {
				continue
			}
			if !found || !templateRequired(v, field.Type) {
				return false
			}
		}
	case reflect.Slice:
		array, ok := value.([]any)
		if !ok {
			return false
		}
		for _, v := range array {
			if !templateRequired(v, typ.Elem()) {
				return false
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return false
		}
		for _, v := range object {
			if !templateRequired(v, typ.Elem()) {
				return false
			}
		}
	}
	return true
}

func templateNoCredentials(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if templateCredential.MatchString(key) || slices.Contains([]string{"__proto__", "prototype", "constructor"}, key) || !templateNoCredentials(item) {
				return false
			}
		}
	case []any:
		for _, item := range v {
			if !templateNoCredentials(item) {
				return false
			}
		}
	case string:
		lower := strings.ToLower(v)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(v, "//") {
			return templateBaseURL(v)
		}
	}
	return true
}

func templateBaseURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.ContainsAny(value, "\\\r\n")
}
func templateEndpoint(value string) bool {
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") && !strings.ContainsAny(value, "?#\\\r\n") && !strings.Contains(value, "{channelKey}")
}
func templatePath(value string) bool {
	if !templateFieldPath.MatchString(value) {
		return false
	}
	for _, key := range strings.Split(value, ".") {
		if slices.Contains([]string{"__proto__", "prototype", "constructor"}, key) {
			return false
		}
	}
	return true
}
func templateUnique(values []string) bool {
	seen := map[string]bool{}
	for _, v := range values {
		if v == "" || seen[v] {
			return false
		}
		seen[v] = true
	}
	return true
}
func validTemplateModel(m model.TemplateModel) bool {
	if !slices.Contains([]string{"image", "video", "audio"}, m.Kind) || !slices.Contains([]string{"POST", "PUT", "PATCH", "GET"}, m.Method) || !templateEndpoint(m.Path) || !slices.Contains([]string{"json", "urlencoded", "formData"}, m.Format) {
		return false
	}
	if !slices.Contains([]string{"bearer", "header", "none"}, m.Auth.Type) || !templateHeader.MatchString(m.Auth.Header) || strings.ContainsAny(m.Auth.Prefix, "\r\n") {
		return false
	}
	if !slices.Contains([]string{"GET", "POST"}, m.Poll.Method) {
		return false
	}
	if m.Poll.Enabled {
		if !templatePath(m.Poll.TaskID) || !templatePath(m.Poll.Status) || (m.Poll.Progress != "" && !templatePath(m.Poll.Progress)) || !templateEndpoint(m.Poll.Path) || !strings.Contains(m.Poll.Path, "{taskId}") || len(m.Poll.Success) == 0 {
			return false
		}
		for _, v := range m.Poll.Success {
			if v == "" || slices.Contains(m.Poll.Failure, v) {
				return false
			}
		}
		for _, v := range m.Poll.Failure {
			if v == "" {
				return false
			}
		}
	} else if m.Content != "" {
		return false
	}
	if m.Content != "" && (!templateEndpoint(m.Content) || !strings.Contains(m.Content, "{taskId}")) {
		return false
	}
	if (m.Result == "" && m.Content == "") || (m.Result != "" && !templatePath(m.Result)) {
		return false
	}
	if _, err := parseTranslationInputs(string(m.Inputs)); err != nil {
		return false
	}
	for i, f := range m.Fields {
		keys := strings.Split(f.Target, ".")
		if !templatePath(f.Target) || templateCredential.MatchString(keys[len(keys)-1]) || !slices.Contains([]string{"model", "rawPrompt", "rawSize", "rawSeconds", "resolution", "quality", "count", "mode", "negativePrompt", "generateAudio", "watermark", "images", "firstImage", "firstFrame", "lastFrame", "videos", "audios", "voice", "audioFormat", "speed", "instructions", "literal"}, f.Source) {
			return false
		}
		if !slices.Contains([]string{"always", "singleImage", "multipleImages", "frames", "noFrames"}, f.When) || !slices.Contains([]string{"body", "query"}, f.Location) || (f.Location == "query" && len(keys) > 1) || (m.Method == "GET" && f.Location == "body") {
			return false
		}
		if f.Source == "literal" {
			var v any
			if json.Unmarshal([]byte(f.Value), &v) != nil || !templateNoCredentials(v) {
				return false
			}
		}
		for j, other := range m.Fields {
			if i != j && f.Location == other.Location && (f.Target == other.Target || strings.HasPrefix(f.Target, other.Target+".")) {
				return false
			}
		}
	}
	return true
}
