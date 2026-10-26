package model

import "encoding/json"

// ConfigJSONLimit retains the existing account-configuration request boundary.
const ConfigJSONLimit = 129 << 20

type ChannelTemplate struct {
	Format            string            `json:"format"`
	Version           int               `json:"version"`
	Name              string            `json:"name"`
	Instructions      string            `json:"instructions,omitempty"`
	BaseURL           string            `json:"baseUrl"`
	Protocol          string            `json:"protocol"`
	Models            []string          `json:"models"`
	ModelCapabilities map[string]string `json:"modelCapabilities"`
	Authoring         TemplateAuthoring `json:"authoring"`
}

type TemplateAuthoring struct {
	Version  int             `json:"version"`
	Models   []TemplateModel `json:"models"`
	Defaults []TemplateModel `json:"defaults,omitempty"`
}

type TemplateModel struct {
	Name   string          `json:"name"`
	Kind   string          `json:"kind"`
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Format string          `json:"format"`
	Fields []TemplateField `json:"fields"`
	Auth   struct {
		Type   string `json:"type"`
		Header string `json:"header"`
		Prefix string `json:"prefix"`
	} `json:"auth"`
	Poll struct {
		Enabled  bool     `json:"enabled"`
		Method   string   `json:"method"`
		Path     string   `json:"path"`
		TaskID   string   `json:"taskId"`
		Status   string   `json:"status"`
		Progress string   `json:"progress"`
		Success  []string `json:"success"`
		Failure  []string `json:"failure"`
	} `json:"poll"`
	Result  string          `json:"result"`
	Content string          `json:"content"`
	Inputs  json.RawMessage `json:"inputs"`
}

type TemplateField struct {
	Target     string `json:"target"`
	Source     string `json:"source"`
	Value      string `json:"value"`
	String     bool   `json:"string"`
	Optional   bool   `json:"optional"`
	OmitValues []any  `json:"omitValues,omitempty"`
	When       string `json:"when"`
	Location   string `json:"location"`
}

type PersonalChannelTemplate struct {
	ID       string          `json:"id"`
	Template json.RawMessage `json:"template"`
}
