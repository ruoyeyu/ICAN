package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tigerowo/infinite-canvas/model"
)

const contractPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jGZkAAAAASUVORK5CYII="

func TestTranslationReferencesAreStaticAndEnforcedBeforeRequests(t *testing.T) {
	for _, source := range []string{
		`{models:{sample:{references:{format:(()=>"url")()},POST:{url:"/generate"}}}}`,
		`{models:{sample:{references:{format:"url",max:-1},POST:{url:"/generate"}}}}`,
		`{models:{sample:{references:{format:"unknown"},POST:{url:"/generate"}}}}`,
	} {
		if _, err := ParameterTranslationModels(source); err == nil {
			t.Fatal("invalid reference declaration accepted")
		}
	}
	source := `{models:{sample:{references:{format:"url",max:2},POST:{url:"/generate",body:{rawPrompt,rawSize,quality,images}}}}}`
	inputs := ParameterTranslationReferenceInputs(source, []string{"sample"})
	if inputs["sample"].Format != "url" || *inputs["sample"].Max != 2 {
		t.Fatal("public input metadata missing")
	}
	for _, refs := range [][]string{{"blob:browser-only"}, {"http://127.0.0.1/private"}, {"https://a.invalid/a", "https://a.invalid/b", "https://a.invalid/c"}} {
		if _, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://upstream.invalid", ParameterTranslation: source}, "sample", map[string]any{"images": refs}); err == nil {
			t.Fatal("invalid references accepted", refs)
		}
	}
}

func TestTranslationMultipartUsesOrderedBinaryImageParts(t *testing.T) {
	data, _ := base64.StdEncoding.DecodeString(contractPNG)
	source := `{models:{sample:{references:{format:"dataUrl"},POST:{url:"/edit",format:"formData",parts:[{name:"model",value:model},{name:"image[]",file:images[0],filename:"reference.png"},{name:"image[]",file:images[1]},{name:"prompt",value:rawPrompt},{name:"size",value:rawSize},{name:"quality",value:"high"}]},result:"data"}}}`
	translation, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://upstream.invalid", APIKey: "fixture-key", ParameterTranslation: source}, "sample", map[string]any{"rawPrompt": "  exact {channelKey} {taskId}\n原文  ", "rawSize": "1537x1001", "images": []string{"data:image/png;base64," + contractPNG, "data:image/png;base64," + contractPNG}})
	if err != nil {
		t.Fatal(err)
	}
	request, _, err := translation.request(context.Background(), false, "")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := request.MultipartReader()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, part.FormName())
		raw, _ := io.ReadAll(part)
		switch part.FormName() {
		case "image[]":
			if part.FileName() == "" || part.Header.Get("Content-Type") != "image/png" || !bytes.Equal(data, raw) {
				t.Fatal("reference was not a binary file")
			}
		case "prompt":
			if string(raw) != "  exact {channelKey} {taskId}\n原文  " {
				t.Fatal("prompt changed")
			}
		case "size":
			if string(raw) != "1537x1001" {
				t.Fatal("custom size changed")
			}
		case "quality":
			if string(raw) != "high" {
				t.Fatal("quality changed")
			}
		}
	}
	if !reflect.DeepEqual(names, []string{"model", "image[]", "image[]", "prompt", "size", "quality"}) {
		t.Fatal("multipart order changed", names)
	}
	if request.Header.Get("Authorization") != "Bearer fixture-key" {
		t.Fatal("wrong authorization")
	}
	invalid := strings.Replace(source, `images[0]`, `"data:image/png;base64,PGh0bWw+eDwvaHRtbD4="`, 1)
	if _, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://upstream.invalid", ParameterTranslation: invalid}, "sample", map[string]any{"images": []string{}}); err == nil {
		t.Fatal("fake image accepted")
	}
}

func TestTranslationExplicitAsyncImageContent(t *testing.T) {
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		body     []byte
		redirect string
		fail     bool
	}{
		{"png", mustContractPNG(), "", false}, {"jpeg", jpegBytes.Bytes(), "", false},
		{"html", []byte("<html>not an image</html>"), "", true},
		{"json", []byte(`{"error":"denied"}`), "", true},
		{"empty", nil, "", true},
		{"same-origin redirect", nil, "https://upstream.invalid/elsewhere", true},
		{"cross-origin redirect", nil, "https://other.invalid/elsewhere", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts, gets := 0, 0
			upstream := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Fatal("missing channel key")
				}
				switch r.URL.Path {
				case "/v1/jobs":
					posts++
					fmt.Fprint(w, `{"task_id":"job"}`)
				case "/v1/jobs/job":
					gets++
					if gets == 1 {
						fmt.Fprint(w, `{"status":"processing","progress":31}`)
					} else {
						fmt.Fprint(w, `{"status":"completed"}`)
					}
				case "/v1/jobs/job/content":
					if r.Method != "GET" || r.ContentLength != 0 || r.Header.Get("Content-Type") != "" {
						t.Fatal("submission body leaked")
					}
					if r.URL.Query().Get("keep") != "base" {
						t.Fatal("query lost")
					}
					if tc.redirect != "" {
						w.Header().Set("Location", tc.redirect)
						w.WriteHeader(302)
						return
					}
					w.Header().Set("Content-Type", "application/octet-stream")
					w.Write(tc.body)
				default:
					t.Fatalf("unexpected request %s", r.URL)
				}
			}))
			defer upstream.Close()
			source := `{headers:{Authorization:"Bearer {channelKey}"},models:{sample:{POST:{url:"/jobs",body:{prompt}},poll:{taskId:r=>r.task_id||r.id,GET:{url:"/jobs/{taskId}"},status:"status",progress:"progress",success:["completed"],failure:["failed","error","cancelled","canceled"]},content:{url:"/jobs/{taskId}/content"},result:r=>r.result||null}}}`
			tr, err := NewParameterTranslation(model.ModelChannel{BaseURL: upstream.URL + "/v1?keep=base", APIKey: "fixture-key", ParameterTranslation: source}, "sample", map[string]any{"prompt": "unchanged"})
			if err != nil {
				t.Fatal(err)
			}
			started, err := tr.Send(context.Background(), "image", false, "")
			if err != nil || len(started.TaskIDs) != 1 {
				t.Fatal(err, started)
			}
			pending, err := tr.Send(context.Background(), "image", true, "job")
			if err != nil || pending.Progress != 31 || pending.Status != "processing" {
				t.Fatal(err, pending)
			}
			result, err := tr.Send(context.Background(), "image", true, "job")
			if (err != nil) != tc.fail {
				t.Fatalf("err=%v fail=%v", err, tc.fail)
			}
			if posts != 1 {
				t.Fatal("task was resubmitted")
			}
			if !tc.fail {
				want := "image/png"
				if tc.name == "jpeg" {
					want = "image/jpeg"
				}
				if len(result.URLs) != 1 || !strings.HasPrefix(result.URLs[0], "data:"+want+";base64,") {
					t.Fatal("wrong image MIME", result.URLs)
				}
			}
		})
	}
}
func mustContractPNG() []byte { data, _ := base64.StdEncoding.DecodeString(contractPNG); return data }

func TestTranslationContentCannotLeakCredentialsOrHideSelectorErrors(t *testing.T) {
	for _, url := range []string{"https://other.invalid/content", "https://user@upstream.invalid/content", "http://upstream.invalid/content"} {
		source := fmt.Sprintf(`{models:{sample:{POST:{url:"/jobs"},poll:{taskId:"id",GET:{url:"/jobs/{taskId}"},status:"status",success:["done"]},content:{url:%q}}}}`, url)
		if _, err := NewParameterTranslation(model.ModelChannel{BaseURL: "https://upstream.invalid", ParameterTranslation: source}, "sample", nil); err == nil {
			t.Fatal("unsafe content URL accepted", url)
		}
	}
	upstream := newProtocolFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/x" {
			t.Fatal("unexpected content fallback")
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "done"})
	}))
	defer upstream.Close()
	source := `{models:{sample:{POST:{url:"/jobs"},poll:{taskId:"id",GET:{url:"/jobs/{taskId}"},status:"status",success:["done"]},result:"wrong.path",content:{url:"/jobs/{taskId}/content"}}}}`
	tr, err := NewParameterTranslation(model.ModelChannel{BaseURL: upstream.URL, ParameterTranslation: source}, "sample", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tr.Send(context.Background(), "image", true, "x"); err == nil {
		t.Fatal("invalid selector did not fail")
	}
}
