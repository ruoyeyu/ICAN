package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/tigerowo/infinite-canvas/model"
	"github.com/tigerowo/infinite-canvas/service"
)

func ChannelTemplates(w http.ResponseWriter, r *http.Request, admin bool) {
	data, err := service.ListChannelTemplates()
	if err != nil {
		Fail(w, err.Error())
		return
	}
	if !admin {
		data.Errors = nil
	}
	w.Header().Set("Cache-Control", "no-store")
	OK(w, data)
}
func ChannelTemplate(w http.ResponseWriter, r *http.Request, id string) {
	data, err := service.GetChannelTemplate(id)
	if err != nil {
		Fail(w, err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	OK(w, data)
}
func SaveChannelTemplate(w http.ResponseWriter, r *http.Request, id string) {
	var request struct {
		Template json.RawMessage `json:"template"`
		Revision string          `json:"revision"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, model.ConfigJSONLimit))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF {
		Fail(w, "模板请求无效")
		return
	}
	data, err := service.SaveChannelTemplate(id, request.Revision, request.Template)
	if err != nil {
		Fail(w, err.Error())
		return
	}
	OK(w, data)
}
func DeleteChannelTemplate(w http.ResponseWriter, r *http.Request, id string) {
	if err := service.DeleteChannelTemplate(id, r.Header.Get("If-Match")); err != nil {
		Fail(w, err.Error())
		return
	}
	OK(w, map[string]bool{"deleted": true})
}
