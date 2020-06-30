/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package handlers

import (
	"encoding/json"
	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net/http"
	"net/http/httptest"
	"prometheus-configmanager/alertmanager/client/mocks"
	"strings"
	"testing"
)

var (
	sampleFileList = []string{"/template/dir/file1.tmpl", "/template/dir/file2.tmpl", "/template/dir/file3.tmpl"}
	sampleRootDir  = "/template/dir/"
)

func TestGetGetTemplateFileHandler(t *testing.T) {
	// Successful Get
	amClient := getTestAMClient()
	tmplClient := &mocks.TemplateClient{}
	tmplClient.On("GetTemplateFile", mock.Anything).Return("test file", nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec := buildTmplContext(nil, http.MethodGet, "/", v1TemplatePath, "file1")

	err := GetGetTemplateFileHandler(amClient, tmplClient)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	tmplClient.AssertExpectations(t)

	// Client Error
	amClient = getTestAMClient()
	tmplClient = &mocks.TemplateClient{}
	tmplClient.On("GetTemplateFile", mock.Anything).Return("test file", nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec = buildTmplContext(nil, http.MethodGet, "/", v1TemplatePath, "not_a_file")

	err = GetGetTemplateFileHandler(amClient, tmplClient)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, "code=400, message=error getting file not_a_file: file does not exist")
}

func TestGetPostTemplateFileHandler(t *testing.T) {
	// Successful Post
	amClient := getTestAMClient()
	amClient.On("AddTemplateFile", mock.Anything).Return(nil)
	tmplClient := &mocks.TemplateClient{}
	tmplClient.On("CreateTemplateFile", mock.Anything, mock.Anything).Return(nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec := buildTmplContext("test text", http.MethodPost, "/", v1TemplatePath, "file4")

	err := GetPostTemplateFileHandler(amClient, tmplClient)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Client Error
	amClient = getTestAMClient()
	amClient.On("AddTemplateFile", mock.Anything).Return(nil)
	tmplClient = &mocks.TemplateClient{}
	tmplClient.On("CreateTemplateFile", mock.Anything).Return("test file", nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec = buildTmplContext(nil, http.MethodPost, "/", v1TemplatePath, "file1")

	err = GetPostTemplateFileHandler(amClient, tmplClient)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, "code=400, message=file file1 already exists")
}

func TestGetPutTemplateFileHandler(t *testing.T) {
	// Successful Post
	amClient := getTestAMClient()
	tmplClient := &mocks.TemplateClient{}
	tmplClient.On("EditTemplateFile", mock.Anything, mock.Anything).Return(nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec := buildTmplContext("test text", http.MethodPut, "/", v1TemplatePath, "file1")

	err := GetPutTemplateFileHandler(amClient, tmplClient)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Client Error
	amClient = getTestAMClient()
	tmplClient = &mocks.TemplateClient{}
	tmplClient.On("EditTemplateFile", mock.Anything).Return("test file", nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec = buildTmplContext(nil, http.MethodPut, "/", v1TemplatePath, "not_a_file")

	err = GetPutTemplateFileHandler(amClient, tmplClient)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, "code=400, message=error editing file not_a_file: file does not exist")
}

func TestGetDeleteTemplateFileHandler(t *testing.T) {
	// Successful Post
	amClient := getTestAMClient()
	amClient.On("RemoveTemplateFile", mock.Anything).Return(nil)
	tmplClient := &mocks.TemplateClient{}
	tmplClient.On("DeleteTemplateFile", mock.Anything, mock.Anything).Return(nil)
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec := buildTmplContext(nil, http.MethodDelete, "/", v1TemplatePath, "file1")

	err := GetDeleteTemplateFileHandler(amClient, tmplClient)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Client Error
	amClient = getTestAMClient()
	amClient.On("RemoveTemplateFile", mock.Anything).Return(nil)
	tmplClient = &mocks.TemplateClient{}
	tmplClient.On("Root").Return(sampleRootDir)
	c, rec = buildTmplContext(nil, http.MethodGet, "/", v1TemplatePath, "not_a_file")

	err = GetDeleteTemplateFileHandler(amClient, tmplClient)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, "code=400, message=error deleting file: file not_a_file does not exist")
}

func getTestAMClient() *mocks.AlertmanagerClient {
	client := mocks.AlertmanagerClient{}
	client.On("GetTemplateFileList").Return(sampleFileList, nil)
	return &client
}

func buildTmplContext(body interface{}, method, target, path, tmplFileName string) (echo.Context, *httptest.ResponseRecorder) {
	bytes, _ := json.Marshal(body)
	req := httptest.NewRequest(method, target, strings.NewReader(string(bytes)))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetPath(path)
	c.Set(templateFilenameParam, tmplFileName)
	return c, rec
}
