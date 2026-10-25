package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
)

var templateFileID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
var templateCatalogWrite sync.Mutex
var errTemplateConflict = errors.New("模板已变更或删除，请刷新后重试")

type ChannelTemplateEntry struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Models   []string               `json:"models"`
	Revision string                 `json:"revision"`
	Template *model.ChannelTemplate `json:"template,omitempty"`
}
type ChannelTemplateCatalog struct {
	Items  []ChannelTemplateEntry `json:"items"`
	Errors []string               `json:"errors,omitempty"`
}

func templateRoot(create bool) (*os.Root, error) {
	dir := config.Cfg.APITemplateDir
	if dir == "" {
		dir = "data/api-templates"
	}
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}
	return os.OpenRoot(dir)
}

func readChannelTemplate(root *os.Root, id string) (ChannelTemplateEntry, error) {
	if !templateFileID.MatchString(id) {
		return ChannelTemplateEntry{}, errTemplate
	}
	original, err := root.Lstat(id + ".json")
	if err != nil || !original.Mode().IsRegular() {
		return ChannelTemplateEntry{}, errTemplate
	}
	file, err := root.OpenFile(id+".json", os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return ChannelTemplateEntry{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(original, info) || info.Size() > model.ConfigJSONLimit {
		return ChannelTemplateEntry{}, errTemplate
	}
	raw, err := io.ReadAll(io.LimitReader(file, model.ConfigJSONLimit+1))
	if err != nil || len(raw) > model.ConfigJSONLimit {
		return ChannelTemplateEntry{}, errTemplate
	}
	template, err := ParseChannelTemplate(raw)
	if err != nil {
		return ChannelTemplateEntry{}, err
	}
	digest := sha256.Sum256(raw)
	return ChannelTemplateEntry{ID: id, Name: template.Name, Models: template.Models, Revision: hex.EncodeToString(digest[:]), Template: &template}, nil
}

func ListChannelTemplates() (ChannelTemplateCatalog, error) {
	out := ChannelTemplateCatalog{Items: []ChannelTemplateEntry{}}
	root, err := templateRoot(false)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, errors.New("无法读取模板目录")
	}
	defer root.Close()
	files, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return out, errors.New("无法读取模板目录")
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(file.Name(), ".json")
		entry, err := readChannelTemplate(root, id)
		if err != nil {
			out.Errors = append(out.Errors, file.Name()+"：格式无效、不是普通文件或无法读取")
			continue
		}
		entry.Template = nil
		out.Items = append(out.Items, entry)
	}
	return out, nil
}

func GetChannelTemplate(id string) (ChannelTemplateEntry, error) {
	root, err := templateRoot(false)
	if err != nil {
		return ChannelTemplateEntry{}, errors.New("模板不可用，请刷新选择器")
	}
	defer root.Close()
	entry, err := readChannelTemplate(root, id)
	if err != nil {
		return ChannelTemplateEntry{}, errors.New("模板不可用，请刷新选择器")
	}
	return entry, nil
}

func SaveChannelTemplate(id, revision string, raw []byte) (ChannelTemplateEntry, error) {
	template, err := ParseChannelTemplate(raw)
	if err != nil {
		return ChannelTemplateEntry{}, err
	}
	encoded, err := json.MarshalIndent(template, "", "  ")
	if err != nil || len(encoded) > model.ConfigJSONLimit {
		return ChannelTemplateEntry{}, errTemplate
	}
	templateCatalogWrite.Lock()
	defer templateCatalogWrite.Unlock()
	root, err := templateRoot(true)
	if err != nil {
		return ChannelTemplateEntry{}, errors.New("无法写入模板目录")
	}
	defer root.Close()
	creating := id == ""
	if creating {
		id = uuid.NewString()
	} else {
		old, err := readChannelTemplate(root, id)
		if err != nil || revision == "" || old.Revision != revision {
			return ChannelTemplateEntry{}, errTemplateConflict
		}
	}
	tmp := uuid.NewString() + ".tmp"
	file, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ChannelTemplateEntry{}, errors.New("无法写入模板目录")
	}
	defer root.Remove(tmp)
	_, err = file.Write(encoded)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return ChannelTemplateEntry{}, errors.New("模板写入失败")
	}
	if creating {
		err = root.Link(tmp, id+".json")
	} else {
		err = root.Rename(tmp, id+".json")
	}
	if err != nil {
		return ChannelTemplateEntry{}, errors.New("模板写入失败")
	}
	return readChannelTemplate(root, id)
}

func DeleteChannelTemplate(id, revision string) error {
	templateCatalogWrite.Lock()
	defer templateCatalogWrite.Unlock()
	root, err := templateRoot(false)
	if err != nil {
		return errTemplateConflict
	}
	defer root.Close()
	old, err := readChannelTemplate(root, id)
	if err != nil || revision == "" || old.Revision != revision {
		return errTemplateConflict
	}
	if root.Remove(id+".json") != nil {
		return errors.New("模板删除失败")
	}
	return nil
}
