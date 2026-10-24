package handler

import (
	"mime/multipart"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/model"
)

func TestRemovedVendorUploadCannotLeakReferences(t *testing.T) {
	blockProtocolNetwork(t)
	url, err := uploadKIEReferenceFile(model.ModelChannel{BaseURL: "https://own-api.invalid", APIKey: "fixture-key"}, &multipart.FileHeader{Filename: "private.png"})
	if err == nil || url != "" || !strings.Contains(err.Error(), "第三方上传已移除") {
		t.Fatal("removed upload must fail explicitly without sending a key or file")
	}
	for _, kind := range []string{"image", "video", "audio"} {
		field := kind + "_reference[]"
		if kind == "image" {
			field = "input_reference[]"
		}
		_, err := prepareDirectAIRequest(directAIRequestInput{
			Channel: directAIChannelInput{Protocol: "kie", BaseURL: "https://own-api.invalid"},
			Model:   "bytedance/seedance-2", Endpoint: "/videos",
			Body: map[string]any{"prompt": "unchanged", field: []string{"https://direct-reference.invalid/run/" + kind + "/0"}},
		})
		if err == nil || !strings.Contains(err.Error(), "第三方上传已移除") {
			t.Fatalf("returned upload instructions for %s: %v", kind, err)
		}
	}
}

func TestRemovedProtocolsCannotUseDirectTranslation(t *testing.T) {
	blockProtocolNetwork(t)
	for _, protocol := range []string{"tokendance", "autodl", "runninghub", "comfyui"} {
		_, err := prepareDirectAIRequest(directAIRequestInput{Channel: directAIChannelInput{Protocol: protocol, BaseURL: "https://own-api.invalid"}, Model: "fixture", Endpoint: "/videos", Body: map[string]any{}})
		if err == nil || !strings.Contains(err.Error(), "未启用") {
			t.Fatalf("accepted disabled protocol %s: %v", protocol, err)
		}
	}
}
