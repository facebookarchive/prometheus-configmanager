/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/facebookincubator/prometheus-configmanager/alertmanager/client"
	"github.com/facebookincubator/prometheus-configmanager/alertmanager/client/mocks"
	"github.com/imdario/mergo"
	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	sampleFileList = []string{"/template/dir/file1.tmpl", "/template/dir/file2.tmpl", "/template/dir/file3.tmpl"}
	sampleRootDir  = "/template/dir/"
)

type templateTestCase struct {
	Name                     string
	Filename                 string
	Payload                  interface{}
	TmplClientFunc           string
	TmplClientExpectedParams []interface{}
	TmplClientExpectedReturn []interface{}

	AmClientFunc           string
	AmClientExpectedParams []interface{}
	AmClientExpectedReturn []interface{}

	HandlerFunc   func(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error
	ExpectedError string
}

func (tc *templateTestCase) RunTest(t *testing.T) {
	tmplClient := getTestTmplClient()
	amClient := getTestAMClient()

	tmplClient.On(tc.TmplClientFunc, tc.TmplClientExpectedParams...).Return(tc.TmplClientExpectedReturn...)
	amClient.On(tc.AmClientFunc, tc.AmClientExpectedParams...).Return(tc.AmClientExpectedReturn...)

	bytes, _ := json.Marshal(tc.Payload)
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(string(bytes)))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetPath(v1TemplatePath)
	c.Set(templateFilenameParam, tc.Filename)
	c.Set(templateNameParam, "test")

	err := tc.HandlerFunc(amClient, tmplClient)(c)
	if tc.ExpectedError == "" {
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		tmplClient.AssertExpectations(t)
		return
	}
	assert.EqualError(t, err, tc.ExpectedError)
}

func TestGetGetTemplateFileHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful get",
		TmplClientFunc:           "GetTemplateFile",
		TmplClientExpectedParams: []interface{}{mock.Anything},
		TmplClientExpectedReturn: []interface{}{"test file", nil},
		HandlerFunc:              GetGetTemplateFileHandler,
		Filename:                 "file1",
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "non-existent file",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error getting file not_a_file: file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{"test file", errors.New("template error")},
			ExpectedError:            "code=500, message=error getting template file: template error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetPostTemplateFileHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful post",
		Filename:                 "file4",
		Payload:                  "test text",
		TmplClientFunc:           "CreateTemplateFile",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{nil},
		AmClientFunc:             "AddTemplateFile",
		AmClientExpectedParams:   []interface{}{mock.Anything},
		AmClientExpectedReturn:   []interface{}{nil},
		HandlerFunc:              GetPostTemplateFileHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file already exists",
			Filename:      "file1",
			ExpectedError: "code=400, message=file file1 already exists",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{errors.New("template error")},
			ExpectedError:            "code=500, message=error creating template file: template error",
		},
		{
			Name:                   "alertmanager client error",
			AmClientExpectedReturn: []interface{}{errors.New("alertmanager error")},
			ExpectedError:          "code=500, message=error creating template file: alertmanager error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetPutTemplateFileHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful post",
		Filename:                 "file1",
		Payload:                  "test text",
		TmplClientFunc:           "EditTemplateFile",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{nil},
		HandlerFunc:              GetPutTemplateFileHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error editing file not_a_file: file does not exist",
		},
		{
			Name:                     "error editing file",
			TmplClientExpectedReturn: []interface{}{errors.New("editing error")},
			ExpectedError:            "code=500, message=error editing template file: editing error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetDeleteTemplateFileHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful post",
		Filename:                 "file1",
		TmplClientFunc:           "DeleteTemplateFile",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{nil},
		AmClientFunc:             "RemoveTemplateFile",
		AmClientExpectedParams:   []interface{}{mock.Anything},
		AmClientExpectedReturn:   []interface{}{nil},
		HandlerFunc:              GetDeleteTemplateFileHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error deleting file: file not_a_file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{errors.New("template error")},
			ExpectedError:            "code=500, message=error deleting template file: template error",
		},
		{
			Name:                   "alertmanager client error",
			AmClientExpectedReturn: []interface{}{errors.New("alertmanager error")},
			ExpectedError:          "code=500, message=error deleting template file: alertmanager error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetGetTemplateHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful get",
		Filename:                 "file1",
		TmplClientFunc:           "GetTemplate",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{"sample template", nil},
		HandlerFunc:              GetGetTemplateHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error getting template: file not_a_file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{"", errors.New("template error")},
			ExpectedError:            "code=500, message=error getting template: template error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetGetTemplatesHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful get",
		Filename:                 "file1",
		TmplClientFunc:           "GetTemplates",
		TmplClientExpectedParams: []interface{}{mock.Anything},
		TmplClientExpectedReturn: []interface{}{map[string]string{"a": "sample template"}, nil},
		HandlerFunc:              GetGetTemplatesHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error getting file: file not_a_file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{map[string]string{"a": "sample template"}, errors.New("template error")},
			ExpectedError:            "code=500, message=error getting templates: template error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetPostTemplateHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful post",
		Filename:                 "file1",
		TmplClientFunc:           "AddTemplate",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{nil},
		HandlerFunc:              GetPostTemplateHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error getting file: file not_a_file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{errors.New("template error")},
			ExpectedError:            "code=500, message=error adding template: template error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetPutTemplateHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful put",
		Filename:                 "file1",
		TmplClientFunc:           "EditTemplate",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{nil},
		HandlerFunc:              GetPutTemplateHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error getting template: file not_a_file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{errors.New("template error")},
			ExpectedError:            "code=500, message=error editing template: template error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func TestGetDeleteTemplateHandler(t *testing.T) {
	baseTest := templateTestCase{
		Name:                     "successful delete",
		Filename:                 "file1",
		TmplClientFunc:           "DeleteTemplate",
		TmplClientExpectedParams: []interface{}{mock.Anything, mock.Anything},
		TmplClientExpectedReturn: []interface{}{nil},
		HandlerFunc:              GetDeleteTemplateHandler,
	}
	tests := []templateTestCase{
		baseTest,
		{
			Name:          "file doesn't exist",
			Filename:      "not_a_file",
			ExpectedError: "code=400, message=error getting template: file not_a_file does not exist",
		},
		{
			Name:                     "template client error",
			TmplClientExpectedReturn: []interface{}{errors.New("template error")},
			ExpectedError:            "code=500, message=error deleting template: template error",
		},
	}
	runAllTests(t, tests, baseTest)
}

func getTestAMClient() *mocks.AlertmanagerClient {
	client := mocks.AlertmanagerClient{}
	client.On("GetTemplateFileList").Return(sampleFileList, nil)
	return &client
}

func getTestTmplClient() *mocks.TemplateClient {
	client := mocks.TemplateClient{}
	client.On("Root").Return(sampleRootDir)
	return &client
}

func runAllTests(t *testing.T, tests []templateTestCase, baseTest templateTestCase) {
	for i := range tests {
		err := mergo.Merge(&tests[i], baseTest)
		assert.NoError(t, err)
	}
	for _, tc := range tests {
		t.Run(tc.Name, tc.RunTest)
	}
}
