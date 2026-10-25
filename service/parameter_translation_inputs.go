package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tigerowo/infinite-canvas/model"
	"io"
	"slices"
)

// Versioned optional metadata lives in the existing translation string. Parsing
// metadata never evaluates JavaScript and never changes a stored channel.
func parseTranslationInputs(source string) (*model.ParameterTranslationInputs, error) {
	if source == "" {
		return nil, nil
	}
	var input model.ParameterTranslationInputs
	decoder := json.NewDecoder(bytes.NewBufferString(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("inputs 必须是静态 JSON：%w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || (input.Authentication != "" && input.Authentication != "key" && input.Authentication != "none") || input.Version != 1 || input.Parameters == nil || input.Media == nil {
		return nil, errors.New("inputs 需要 version:1、parameters 和 media")
	}
	var raw struct {
		Parameters map[string]map[string]json.RawMessage `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(source), &raw); err != nil {
		return nil, err
	}
	for name, option := range input.Parameters {
		fields := raw.Parameters[name]
		if len(fields["allowCustom"]) == 0 || string(fields["allowCustom"]) == "null" || len(fields["options"]) == 0 {
			return nil, errors.New("每组参数需要明确的 options 和 allowCustom")
		}

		if !slices.Contains([]string{"seconds", "resolution", "aspectRatio", "mode"}, name) || option.Options == nil || (!option.AllowCustom && len(option.Options) == 0) {
			return nil, errors.New("参数选项声明无效")
		}
		seen := map[string]bool{}
		for _, value := range option.Options {
			if value == "" || seen[value] {
				return nil, errors.New("参数选项不能空白或重复")
			}
			seen[value] = true
		}
	}
	for name, policy := range input.Media {
		if !slices.Contains([]string{"images", "firstFrame", "lastFrame", "videos", "audios"}, name) || !slices.Contains([]string{"auto", "url", "dataUrl"}, policy.Format) || (policy.Max != nil && *policy.Max < 0) {
			return nil, errors.New("素材输入声明无效")
		}
		if (name == "videos" || name == "audios") && policy.Format == "dataUrl" {
			return nil, errors.New("视频和音频输入目前支持 URL 或保留原编码")
		}
	}
	return &input, nil
}

func ParameterTranslationModelInputs(source string, names []string) map[string]model.ParameterTranslationInputs {
	programs, err := parameterTranslationPrograms(source)
	if err != nil {
		return nil
	}
	result := map[string]model.ParameterTranslationInputs{}
	for _, name := range names {
		if p := programs[name]; p != nil && p.inputs != nil {
			result[name] = *p.inputs
		}
	}
	return result
}

func validateTranslationInputs(input *model.ParameterTranslationInputs, variables map[string]any) error {
	if input == nil {
		return nil
	}
	for name, option := range input.Parameters {
		variable := name
		if name == "seconds" {
			variable = "rawSeconds"
		}
		if name == "aspectRatio" {
			variable = "rawSize"
		}
		value, ok := variables[variable].(string)
		if !ok || value == "" || (!option.AllowCustom && !slices.Contains(option.Options, value)) {
			return fmt.Errorf("%s 参数不在该模型声明的范围内，请检查输入", name)
		}
	}
	for _, name := range []string{"images", "firstFrame", "lastFrame", "videos", "audios"} {
		if _, declared := input.Media[name]; declared {
			continue
		}
		supplied := false
		switch value := variables[name].(type) {
		case nil:
		case string:
			supplied = value != ""
		case []string:
			supplied = len(value) > 0
		case []any:
			supplied = len(value) > 0
		default:
			supplied = true
		}
		if supplied {
			return fmt.Errorf("当前模型未声明 %s 输入，请先配置素材映射与编码", name)
		}
	}
	for name, policy := range input.Media {
		value := variables[name]
		if name == "firstFrame" || name == "lastFrame" {
			text, ok := value.(string)
			if value != nil && !ok {
				return fmt.Errorf("%s 必须是素材地址", name)
			}
			value = []string{}
			if text != "" {
				value = []string{text}
			}
		} else if value == nil {
			value = []string{}
		}
		if err := validateTranslationReferences(&policy, value); err != nil {
			return fmt.Errorf("%s：%w", name, err)
		}
	}
	return nil
}

// Only an explicit declaration on this exact model permits an empty provider key.
func ParameterTranslationWithoutKey(source, name string) bool {
	programs, err := parameterTranslationPrograms(source)
	return err == nil && programs[name] != nil && programs[name].inputs != nil && programs[name].inputs.Authentication == "none"
}
