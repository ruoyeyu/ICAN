package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/tigerowo/infinite-canvas/model"
)

// Reference declarations are static metadata, so listing channels never runs a template.
func parseTranslationReferences(expression ast.Expression) (*model.ParameterTranslationReferences, error) {
	if expression == nil {
		return nil, nil
	}
	object, ok := expression.(*ast.ObjectLiteral)
	if !ok {
		return nil, errors.New("references 必须是静态对象")
	}
	fields, err := parameterTranslationProperties(object)
	if err != nil {
		return nil, err
	}
	result := &model.ParameterTranslationReferences{}
	for name, value := range fields {
		switch name {
		case "format":
			literal, ok := value.(*ast.StringLiteral)
			if !ok {
				return nil, errors.New("references.format 必须是静态字符串")
			}
			result.Format = literal.Value.String()
		case "max":
			literal, ok := value.(*ast.NumberLiteral)
			if !ok {
				return nil, errors.New("references.max 必须是非负整数")
			}
			maximum, err := strconv.Atoi(literal.Literal)
			if err != nil || maximum < 0 {
				return nil, errors.New("references.max 必须是非负整数")
			}
			result.Max = &maximum
		default:
			return nil, fmt.Errorf("不支持的 references 字段：%s", name)
		}
	}
	if result.Format != "url" && result.Format != "dataUrl" && result.Format != "auto" {
		return nil, errors.New("references.format 只能是 url、dataUrl 或 auto")
	}
	return result, nil
}

func ParameterTranslationReferenceInputs(source string, names []string) map[string]model.ParameterTranslationReferences {
	programs, err := parameterTranslationPrograms(source)
	if err != nil {
		return nil
	}
	result := map[string]model.ParameterTranslationReferences{}
	for _, name := range names {
		if program := programs[name]; program != nil && program.references != nil {
			result[name] = *program.references
		}
	}
	return result
}

func validateTranslationReferences(policy *model.ParameterTranslationReferences, value any) error {
	if policy == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return errors.New("参考图片格式错误")
	}
	var images []string
	if err := json.Unmarshal(encoded, &images); err != nil {
		return errors.New("参考图片必须是地址数组")
	}
	if policy.Max != nil && len(images) > *policy.Max {
		return fmt.Errorf("该模型最多接受 %d 张参考图片", *policy.Max)
	}
	for _, image := range images {
		switch policy.Format {
		case "url":
			if err := ValidateOutboundURL(image); err != nil {
				return errors.New("该模型的参考图片需要供应商可读取的 HTTP(S) 链接")
			}
		case "dataUrl":
			if _, _, err := translationImageData(image); err != nil {
				return err
			}
		}
	}
	return nil
}

func translationImageData(value string) ([]byte, string, error) {
	if !strings.HasPrefix(value, "data:image/") || !strings.Contains(strings.SplitN(value, ",", 2)[0], ";base64") {
		return nil, "", errors.New("参考图片需要 image base64 Data URL")
	}
	data, declared, err := readWorkflowMedia(context.Background(), value, 0)
	if err != nil {
		return nil, "", err
	}
	actual, err := PassiveMediaType(data)
	if err != nil || !strings.HasPrefix(actual, "image/") || actual != declared {
		return nil, "", errors.New("参考图片内容与 MIME 类型不匹配或不是有效图片")
	}
	return data, actual, nil
}

// Ordered multipart parts distinguish a binary file from a text field explicitly.
type parameterTranslationPart struct {
	Name     string          `json:"name"`
	Value    json.RawMessage `json:"value"`
	File     string          `json:"file"`
	Filename string          `json:"filename"`
}

func encodeTranslationParts(parts []parameterTranslationPart, replace func(string) string) ([]byte, string, error) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for _, part := range parts {
		if part.Name == "" || strings.ContainsAny(part.Name+part.Filename, "\r\n\x00") {
			return nil, "", errors.New("multipart 字段或文件名无效")
		}
		if part.File != "" {
			if len(part.Value) != 0 {
				return nil, "", errors.New("multipart 段不能同时指定 file 和 value")
			}
			data, kind, err := translationImageData(part.File)
			if err != nil {
				return nil, "", err
			}
			filename := part.Filename
			if filename == "" {
				filename = "reference" + workflowExtension(kind)
			}
			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": part.Name, "filename": filename}))
			header.Set("Content-Type", kind)
			field, err := writer.CreatePart(header)
			if err != nil {
				return nil, "", err
			}
			if _, err = field.Write(data); err != nil {
				return nil, "", err
			}
		} else {
			if len(part.Value) == 0 {
				return nil, "", errors.New("multipart 段需要 value 或 file")
			}
			var value any
			if err := json.Unmarshal(part.Value, &value); err != nil {
				return nil, "", err
			}
			text, ok := value.(string)
			if !ok {
				text = string(part.Value)
			}
			if err := writer.WriteField(part.Name, replace(text)); err != nil {
				return nil, "", err
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), writer.FormDataContentType(), nil
}

func translationContentURL(baseURL, configured, taskID string, source *url.URL) (*url.URL, error) {
	target, err := url.Parse(strings.ReplaceAll(configured, "{taskId}", strings.ReplaceAll(url.QueryEscape(taskID), "+", "%20")))
	if err != nil {
		return nil, errors.New("图片内容下载地址无效")
	}
	if !target.IsAbs() {
		base, err := url.Parse(baseURL)
		if err != nil {
			return nil, errors.New("渠道地址无效")
		}
		path := strings.TrimRight(base.EscapedPath(), "/") + "/" + strings.TrimLeft(target.EscapedPath(), "/")
		base.Path, err = url.PathUnescape(path)
		if err != nil {
			return nil, err
		}
		base.RawPath, base.RawQuery, base.Fragment = path, target.RawQuery, target.Fragment
		target = base
	}
	if target.User != nil || target.Fragment != "" || target.Scheme != source.Scheme || target.Host != source.Host {
		return nil, errors.New("带鉴权的图片内容下载必须与查询接口同源")
	}
	return target, nil
}

func (translation *ParameterTranslation) configuredImageContent(taskID string, source *url.URL) (*url.URL, error) {
	configured := parameterTranslationValue(translation.rule, "content")
	if goja.IsUndefined(configured) || goja.IsNull(configured) {
		return nil, nil
	}
	value, err := translation.jsonValue(configured)
	if err != nil {
		return nil, err
	}
	var plan struct {
		URL string `json:"url"`
	}
	encoded, _ := json.Marshal(value)
	if json.Unmarshal(encoded, &plan) != nil || plan.URL == "" {
		return nil, errors.New("content 需要明确的 url")
	}
	return translationContentURL(translation.channel.BaseURL, plan.URL, taskID, source)
}
