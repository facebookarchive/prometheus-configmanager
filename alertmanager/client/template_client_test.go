package client

import (
	"github.com/stretchr/testify/assert"
	"prometheus-configmanager/fsclient/mocks"
	"prometheus-configmanager/prometheus/alert"
	"testing"

	"github.com/stretchr/testify/mock"
)

const testTemplateFile = `
{{ define "slack.myorg.text" }}https://internal.myorg.net/wiki/alerts/{{ .GroupLabels.app }}/{{ .GroupLabels.alertname }}{{ end}}
`

func TestTemplateClient_GetTemplateFile(t *testing.T) {
	client, _ := newTestTmplClient()

	fileText, err := client.GetTemplateFile("")
	assert.NoError(t, err)
	assert.Equal(t, testTemplateFile, fileText)
}

func TestTemplateClient_CreateTemplateFile(t *testing.T) {
	client, _ := newTestTmplClient()

	err := client.CreateTemplateFile("test", "text")
	assert.NoError(t, err)
}

func TestTemplateClient_EditTemplateFile(t *testing.T) {
	client, _ := newTestTmplClient()

	err := client.EditTemplateFile("test", "text")
	assert.NoError(t, err)
}

func TestTemplateClient_DeleteTemplateFile(t *testing.T) {
	client, _ := newTestTmplClient()

	err := client.DeleteTemplateFile("test")
	assert.NoError(t, err)
}

func newTestTmplClient() (TemplateClient, *mocks.FSClient) {
	fsClient := &mocks.FSClient{}
	fsClient.On("ReadFile", mock.Anything).Return([]byte(testTemplateFile), nil)
	fsClient.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	fsClient.On("DeleteFile", mock.Anything).Return(nil)
	fileLocks, _ := alert.NewFileLocker(alert.NewDirectoryClient("."))
	return NewTemplateClient(fsClient, fileLocks), fsClient
}
