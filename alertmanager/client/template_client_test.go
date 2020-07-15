/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package client

import (
	"io/ioutil"
	"github.com/facebookincubator/prometheus-configmanager/fsclient/mocks"
	"github.com/facebookincubator/prometheus-configmanager/prometheus/alert"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
)

func TestTemplateClient_GetTemplateFile(t *testing.T) {
	client, _, _ := newTestTmplClient()

	fileText, err := client.GetTemplateFile("")
	assert.NoError(t, err)
	origFile, _ := readTestFileString()
	assert.Equal(t, origFile, fileText)
}

func TestTemplateClient_CreateTemplateFile(t *testing.T) {
	client, _, _ := newTestTmplClient()

	err := client.CreateTemplateFile("test", "text")
	assert.NoError(t, err)
}

func TestTemplateClient_EditTemplateFile(t *testing.T) {
	client, _, out := newTestTmplClient()

	err := client.EditTemplateFile("test", "text")
	assert.NoError(t, err)
	assert.Equal(t, "text", string(*out))
}

func TestTemplateClient_DeleteTemplateFile(t *testing.T) {
	client, _, _ := newTestTmplClient()

	err := client.DeleteTemplateFile("test")
	assert.NoError(t, err)
}

func TestTemplateClient_GetTemplates(t *testing.T) {
	client, _, _ := newTestTmplClient()

	tmpls, err := client.GetTemplates("test")
	assert.NoError(t, err)

	assert.Len(t, tmpls, 2)
	assert.NotNil(t, tmpls["slack.myorg.text"])
	assert.NotNil(t, tmpls["slack.myorg2.text"])
}

func TestTemplateClient_GetTemplate(t *testing.T) {
	client, _, _ := newTestTmplClient()

	const expectedText = `https://internal.myorg.net/wiki/alerts/{{.GroupLabels.app}}/{{.GroupLabels.alertname}}`

	text, err := client.GetTemplate("test", "slack.myorg.text")
	assert.NoError(t, err)
	assert.Equal(t, expectedText, text)

	_, err = client.GetTemplate("test", "noTemplate")
	assert.EqualError(t, err, "template noTemplate not found")
}

func TestTemplateClient_AddTemplate(t *testing.T) {
	client, _, out := newTestTmplClient()

	err := client.AddTemplate("test", "slack2", "test slack body")
	assert.NoError(t, err)
	origFile, _ := readTestFileString()
	expectedOutput := origFile + `
{{ define "slack2" }}test slack body{{ end }}
`
	assert.Equal(t, expectedOutput, string(*out))
}

func TestTemplateClient_EditTemplate(t *testing.T) {
	client, _, out := newTestTmplClient()

	err := client.EditTemplate("test", "slack.myorg.text", "new text")

	expectedText := `{{ define "slack.myorg.text" }}new text{{ end }}
{{ define "slack.myorg2.text" }}https://external.myorg.net/wiki/alerts/{{.GroupLabels.app}}/{{.GroupLabels.alertname}}{{ end }}
`
	assert.NoError(t, err)
	assert.Equal(t, expectedText, string(*out))
}

func TestTemplateClient_DeleteTemplate(t *testing.T) {
	client, _, out := newTestTmplClient()

	err := client.DeleteTemplate("test", "slack.myorg.text")
	assert.NoError(t, err)

	testFile, err := readTestFileString()
	assert.NoError(t, err)
	// Expected to remove first definition
	expectedText := testFile[strings.Index(testFile, `{{ define "slack.myorg2.text" }}`):]
	assert.Equal(t, expectedText, strings.TrimSpace(string(*out)))

	err = client.DeleteTemplate("test", "notATemplate")
	assert.EqualError(t, err, "template notATemplate does not exist")
}

func newTestTmplClient() (TemplateClient, *mocks.FSClient, *[]byte) {
	fsClient := &mocks.FSClient{}
	fsClient.On("ReadFile", mock.Anything).Return(readTestFile())

	var outputFile []byte
	fsClient.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) { outputFile = args[1].([]byte) })

	fsClient.On("DeleteFile", mock.Anything).Return(nil)
	fsClient.On("Root").Return("testdata/")
	fileLocks, _ := alert.NewFileLocker(alert.NewDirectoryClient("."))
	return NewTemplateClient(fsClient, fileLocks), fsClient, &outputFile
}

const copyrightHeaderLength = 200

func readTestFile() ([]byte, error) {
	text, err := ioutil.ReadFile("testdata/test.tmpl")
	return text[copyrightHeaderLength:], err
}

func readTestFileString() (string, error) {
	file, err := ioutil.ReadFile("testdata/test.tmpl")
	return string(file[copyrightHeaderLength:]), err
}
