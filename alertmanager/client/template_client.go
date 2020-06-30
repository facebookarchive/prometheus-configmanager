package client

import (
	"prometheus-configmanager/fsclient"
	"prometheus-configmanager/prometheus/alert"
)

const TemplateFilePostfix = ".tmpl"

// TemplateClient interface provides methods for modifying template files
// and individual templates within them
type TemplateClient interface {
	GetTemplateFile(filename string) (string, error)
	CreateTemplateFile(filename, fileText string) error
	EditTemplateFile(filename, fileText string) error
	DeleteTemplateFile(filename string) error

	Root() string
}

func NewTemplateClient(fsClient fsclient.FSClient, fileLocks *alert.FileLocker) TemplateClient {
	return &templateClient{
		fsClient:  fsClient,
		fileLocks: fileLocks,
	}
}

type templateClient struct {
	templateDir string
	fsClient    fsclient.FSClient
	fileLocks   *alert.FileLocker
}

func (t *templateClient) GetTemplateFile(filename string) (string, error) {
	t.fileLocks.RLock(filename)
	defer t.fileLocks.RUnlock(filename)

	file, err := t.fsClient.ReadFile(addFilePostfix(filename))
	if err != nil {
		return "", err
	}

	return string(file), nil
}

func (t *templateClient) CreateTemplateFile(filename, fileText string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	return t.fsClient.WriteFile(addFilePostfix(filename), []byte(fileText), 0660)
}

func (t *templateClient) EditTemplateFile(filename, fileText string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	return t.fsClient.WriteFile(addFilePostfix(filename), []byte(fileText), 0660)
}

func (t *templateClient) DeleteTemplateFile(filename string) error {
	t.fileLocks.Lock(filename)
	defer t.fileLocks.Unlock(filename)

	return t.fsClient.DeleteFile(addFilePostfix(filename))
}

func (t *templateClient) Root() string {
	return t.fsClient.Root()
}

func addFilePostfix(filename string) string {
	return filename + TemplateFilePostfix
}
