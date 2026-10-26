package service

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// Error diagnostics only; successful response/media limits are unchanged.
const UpstreamErrorBodyLimit = 256 * 1024
const upstreamErrorTextLimit = 2048

var upstreamURL = regexp.MustCompile(`(?i)https?(?::|%3a)(?:/|%2f){2}[^\s"'<>\\]+`)
var upstreamCredential = regexp.MustCompile(`(?i)(?:bearer|basic)\s+[^\s"'<>;,\\]+|\bsk-[a-z0-9_-]+|(?:authorization|api[_-]?key|access[_-]?token|secret|cookie|password|signature)["'\\\s]*[:=]["'\\\s]*[^\r\n,;}<>]+`)
var upstreamRelativeGrant = regexp.MustCompile(`(?i)(?:/|%2f)[^\s"'<>]*[?&](?:[^\s"'<>]*&)?(?:token|sig|signature|key|x-amz-[a-z-]+)=[^\s"'<>]+`)
var upstreamDataURL = regexp.MustCompile(`(?i)data:[^\s"'<>]+`)
var upstreamSecretField = regexp.MustCompile(`(?i)^(?:authorization|proxy[-_]?authorization|x[-_]?api[-_]?key|api[_-]?key|(?:access|refresh|id|session)[_-]?token|token|secret|client[_-]?secret|cookie|set[-_]?cookie|password|signature|credentials?|key|x-amz-(?:credential|signature|security-token))$`)

type UpstreamError struct {
	Status    int    `json:"status"`
	Stage     string `json:"stage"`
	RequestID string `json:"requestId,omitempty"`
	Detail    string `json:"detail"`
	Body      string `json:"body"`
	Truncated bool   `json:"truncated"`
}

func (e *UpstreamError) Error() string {
	context := ""
	if e.Status > 0 {
		context = fmt.Sprintf("HTTP %d", e.Status)
	}
	if e.RequestID != "" {
		if context != "" {
			context += "；"
		}
		context += "request ID: " + e.RequestID
	}
	if context != "" {
		context = "（" + context + "）"
	}
	return "上游" + e.Stage + "失败" + context + "：" + e.Detail
}
func (e *UpstreamError) SafeMessage() string { return e.Error() }
func (e *UpstreamError) Diagnostics() string {
	value, _ := json.MarshalIndent(e, "", "  ")
	return string(value)
}

func decodeUpstreamJSON(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid JSON tail")
	}
	return value, nil
}

func decodeUpstreamValue(value any, depth int) any {
	for depth < 8 {
		text, ok := value.(string)
		if !ok {
			break
		}
		decoded, err := decodeUpstreamJSON(text)
		if err != nil {
			break
		}
		value = decoded
		depth++
	}
	return value
}

func upstreamDetail(value any, depth int) string {
	if depth >= 8 {
		return ""
	}
	value = decodeUpstreamValue(value, depth)
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		for _, item := range v {
			if message := upstreamDetail(item, depth+1); message != "" {
				return message
			}
		}
	case map[string]any:
		if failure, ok := decodeUpstreamValue(v["error"], depth+1).(map[string]any); ok {
			if message := upstreamDetail(failure["message"], depth+1); message != "" {
				return message
			}
		}
		for _, key := range []string{"detail", "message", "msg", "error", "data", "response", "body"} {
			if message := upstreamDetail(v[key], depth+1); message != "" {
				return message
			}
		}
	}
	return ""
}

func upstreamRequestID(value any, depth int) string {
	if depth >= 8 {
		return ""
	}
	v, ok := decodeUpstreamValue(value, depth).(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"request_id", "requestId", "request-id", "trace_id"} {
		if id, ok := v[key].(string); ok && strings.TrimSpace(id) != "" {
			return id
		}
	}
	for _, key := range []string{"error", "detail", "message", "data", "response", "body"} {
		if id := upstreamRequestID(v[key], depth+1); id != "" {
			return id
		}
	}
	return ""
}

func RedactUpstreamText(text string, secrets ...string) string {
	text = redactUpstreamString(text, secrets...)
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > upstreamErrorTextLimit {
		return string(runes[:upstreamErrorTextLimit]) + "…"
	}
	return text
}

func redactUpstreamString(text string, secrets ...string) string {
	text = html.UnescapeString(strings.ReplaceAll(text, `\/`, `/`))
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		encoded, _ := json.Marshal(secret)
		for _, value := range []string{secret, string(encoded[1 : len(encoded)-1]), url.QueryEscape(secret), url.PathEscape(secret), url.QueryEscape(url.QueryEscape(secret))} {
			text = strings.ReplaceAll(text, value, "[已脱敏]")
		}
	}
	text = upstreamURL.ReplaceAllString(text, "[链接已隐藏]")
	text = upstreamRelativeGrant.ReplaceAllString(text, "[链接已隐藏]")
	text = upstreamDataURL.ReplaceAllString(text, "[素材已隐藏]")
	text = upstreamCredential.ReplaceAllString(text, "[凭据已隐藏]")
	return text
}

func redactUpstreamValue(value any, depth int, secrets ...string) any {
	if depth >= 32 {
		return "[嵌套内容已隐藏]"
	}
	switch v := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(v))
		for key, item := range v {
			name := redactUpstreamString(key, secrets...)
			if upstreamSecretField.MatchString(key) {
				clean[name] = "[已脱敏]"
			} else {
				clean[name] = redactUpstreamValue(item, depth+1, secrets...)
			}
		}
		return clean
	case []any:
		for i, item := range v {
			v[i] = redactUpstreamValue(item, depth+1, secrets...)
		}
	case string:
		if nested, err := decodeUpstreamJSON(v); err == nil {
			if _, ok := nested.(string); ok || strings.HasPrefix(strings.TrimSpace(v), "{") || strings.HasPrefix(strings.TrimSpace(v), "[") {
				encoded, _ := json.Marshal(redactUpstreamValue(nested, depth+1, secrets...))
				return string(encoded)
			}
		}
		return redactUpstreamString(v, secrets...)
	}
	return value
}

// Details are for the authenticated caller; public logs retain metadata only.
func NewUpstreamError(body []byte, status int, stage string, headers http.Header, request *http.Request, secrets ...string) *UpstreamError {
	if request != nil {
		for name, values := range request.Header {
			name = strings.ToLower(name)
			if strings.Contains(name, "authorization") || strings.Contains(name, "key") || strings.Contains(name, "token") || strings.Contains(name, "cookie") {
				secrets = append(secrets, values...)
			}
		}
	}
	truncated := len(body) > UpstreamErrorBodyLimit
	if truncated {
		body = body[:UpstreamErrorBodyLimit]
	}
	value, decodeErr := decodeUpstreamJSON(string(body))
	validJSON := decodeErr == nil
	if !validJSON {
		value = string(body)
		if truncated {
			text := value.(string)
			for _, secret := range secrets {
				for n := len(secret) - 1; n > 0; n-- {
					if strings.HasSuffix(text, secret[:n]) {
						text = strings.TrimSuffix(text, secret[:n]) + "[已脱敏]"
						break
					}
				}
			}
			value = text
		}
	}
	detail := RedactUpstreamText(upstreamDetail(value, 0), secrets...)
	if detail == "" {
		detail = "接口没有提供有效错误详情"
		if status == 401 || status == 403 {
			detail = "鉴权失败，接口没有提供有效错误详情"
		}
	}
	id := ""
	for _, name := range []string{"X-Request-ID", "Request-ID", "X-Correlation-ID", "X-Amzn-Requestid", "CF-Ray"} {
		if id = headers.Get(name); strings.TrimSpace(id) != "" {
			break
		}
	}
	if strings.TrimSpace(id) == "" {
		id = upstreamRequestID(value, 0)
	}
	id = RedactUpstreamText(id, secrets...)
	if len([]rune(id)) > 128 {
		id = string([]rune(id)[:128]) + "…"
	}
	full := redactUpstreamValue(value, 0, secrets...)
	fullBody, _ := full.(string)
	if validJSON {
		encoded, _ := json.MarshalIndent(full, "", "  ")
		fullBody = string(encoded)
	}
	if truncated {
		fullBody += "\n[已截断：上游错误正文超过 256 KiB]"
	}
	return &UpstreamError{Status: status, Stage: stage, RequestID: id, Detail: detail, Body: fullBody, Truncated: truncated}
}
