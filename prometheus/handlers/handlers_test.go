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

	"github.com/facebookincubator/prometheus-configmanager/prometheus/alert"
	"github.com/facebookincubator/prometheus-configmanager/prometheus/alert/mocks"

	"github.com/labstack/echo"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDuration, _ = model.ParseDuration("5s")
	longDuration, _   = model.ParseDuration("1w1d")

	sampleAlert1 = rulefmt.Rule{
		Alert:       "testAlert1",
		For:         sampleDuration,
		Expr:        "up == 0",
		Labels:      map[string]string{"label": "value"},
		Annotations: map[string]string{"annotation": "value"},
	}
	sampleInvalidAlert = rulefmt.Rule{
		Alert: "testInvalidAlert",
		Expr:  "invalid{",
	}
	sampleAlert2 = rulefmt.Rule{
		Alert:       "testAlert2",
		For:         sampleDuration,
		Expr:        "up == 0",
		Labels:      map[string]string{"label": "value"},
		Annotations: map[string]string{"annotation": "value"},
	}
	sampleJSONRule1 = alert.RuleJSONWrapper{
		Alert:       "testAlert1",
		Expr:        "up == 0",
		For:         "5s",
		Labels:      map[string]string{"label": "value"},
		Annotations: map[string]string{"annotation": "value"},
	}
	sampleJSONRule2 = alert.RuleJSONWrapper{
		Alert:       "testAlert2",
		Expr:        "up == 0",
		For:         "5s",
		Labels:      map[string]string{"label": "value"},
		Annotations: map[string]string{"annotation": "value"},
	}
	sampleLongDurationRule = rulefmt.Rule{
		Alert: "testLongDurationAlert",
		For:   longDuration,
		Expr:  "up",
	}
)

const (
	testNID = "test"
)

func TestGetConfigureAlertHandler(t *testing.T) {
	// Successful Post
	client := &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(false)
	client.On("WriteRule", testNID, sampleAlert1).Return(nil)
	client.On("ReloadPrometheus").Return(nil)
	c, rec := buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err := GetConfigureAlertHandler(client)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	client.AssertExpectations(t)

	// Rule validation fails
	client = &mocks.PrometheusAlertClient{}
	c, _ = buildContext(sampleInvalidAlert, http.MethodPost, "/", v1alertPath, testNID)

	err = GetConfigureAlertHandler(client)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=400, message=Rule Validation Error; could not parse expression: 1:9: parse error: unexpected end of input inside braces`)
	client.AssertExpectations(t)

	// Rule already exists
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(true)
	c, _ = buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err = GetConfigureAlertHandler(client)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=400, message=Rule 'testAlert1' already exists`)
	client.AssertExpectations(t)

	// Write fails
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(false)
	client.On("WriteRule", testNID, sampleAlert1).Return(errors.New("error"))
	c, _ = buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err = GetConfigureAlertHandler(client)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)

	// Reload Prometheus fails
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(false)
	client.On("WriteRule", testNID, sampleAlert1).Return(nil)
	client.On("ReloadPrometheus").Return(errors.New("error"))
	c, _ = buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err = GetConfigureAlertHandler(client)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)
}

func TestGetRetrieveAlertHandler(t *testing.T) {
	// Successful Get
	client := &mocks.PrometheusAlertClient{}
	client.On("ReadRules", testNID, "").Return([]rulefmt.Rule{sampleAlert1}, nil)
	c, rec := buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err := GetRetrieveAlertHandler(client)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	client.AssertExpectations(t)

	// reads rule with long duration
	client.On("ReadRules", testNID, "").Return([]rulefmt.Rule{sampleLongDurationRule}, nil)
	c, rec = buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err = GetRetrieveAlertHandler(client)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	client.AssertExpectations(t)

	// Error reading rules
	client = &mocks.PrometheusAlertClient{}
	client.On("ReadRules", testNID, "").Return(nil, errors.New("error"))
	c, _ = buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)

	err = GetRetrieveAlertHandler(client)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)
}

func TestGetDeleteAlertHandler(t *testing.T) {
	// Successful Delete
	client := &mocks.PrometheusAlertClient{}
	client.On("DeleteRule", testNID, sampleAlert1.Alert).Return(nil)
	client.On("ReloadPrometheus").Return(nil)

	c, rec := buildContext(nil, http.MethodDelete, "/", v1alertPath, testNID)
	c.SetParamNames(ruleNameParam)
	c.SetParamValues(sampleAlert1.Alert)

	err := GetDeleteAlertHandler(client, pathAlertNameProvider)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	client.AssertExpectations(t)

	// No alert name given
	client = &mocks.PrometheusAlertClient{}
	c, _ = buildContext(nil, http.MethodDelete, "/", v1alertPath, testNID)

	err = GetDeleteAlertHandler(client, pathAlertNameProvider)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	client.AssertExpectations(t)

	// Delete failed in client
	client = &mocks.PrometheusAlertClient{}
	client.On("DeleteRule", testNID, sampleAlert1.Alert).Return(errors.New("error"))
	c, _ = buildContext(nil, http.MethodDelete, "/", v1alertPath, testNID)
	c.SetParamNames(ruleNameParam)
	c.SetParamValues(sampleAlert1.Alert)

	err = GetDeleteAlertHandler(client, pathAlertNameProvider)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)

	// Prometheus reload failed
	client = &mocks.PrometheusAlertClient{}
	client.On("DeleteRule", testNID, sampleAlert1.Alert).Return(nil)
	client.On("ReloadPrometheus").Return(errors.New("error"))
	c, _ = buildContext(nil, http.MethodDelete, "/", v1alertPath, testNID)
	c.SetParamNames(ruleNameParam)
	c.SetParamValues(sampleAlert1.Alert)

	err = GetDeleteAlertHandler(client, pathAlertNameProvider)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)
}

func TestUpdateAlertHandler(t *testing.T) {
	// Successful Update
	client := &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(true)
	client.On("UpdateRule", testNID, sampleAlert1).Return(nil)
	client.On("ReloadPrometheus").Return(nil)
	c, rec := buildContext(sampleAlert1, http.MethodPut, "/", v1alertPath, testNID)
	c.SetParamNames("file_prefix", ruleNameParam)
	c.SetParamValues(testNID, sampleAlert1.Alert)

	err := GetUpdateAlertHandler(client)(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	client.AssertExpectations(t)

	// No rule name provided
	client = &mocks.PrometheusAlertClient{}
	c, _ = buildContext(sampleAlert1, http.MethodPut, "/", v1alertPath, testNID)

	err = GetUpdateAlertHandler(client)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=400, message=No rule name provided`)
	client.AssertExpectations(t)

	// Rule does not exist
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(false)
	c, _ = buildContext(sampleAlert1, http.MethodPut, "/", v1alertPath, testNID)
	c.SetParamNames("file_prefix", ruleNameParam)
	c.SetParamValues(testNID, sampleAlert1.Alert)

	err = GetUpdateAlertHandler(client)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=400, message=Rule 'testAlert1' does not exist`)
	client.AssertExpectations(t)

	// Validate rule fails
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleInvalidAlert.Alert).Return(true)
	c, _ = buildContext(sampleInvalidAlert, http.MethodPut, "/", v1alertPath, testNID)
	c.SetParamNames("file_prefix", ruleNameParam)
	c.SetParamValues(testNID, sampleInvalidAlert.Alert)

	err = GetUpdateAlertHandler(client)(c)
	assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=400, message=Rule Validation Error; could not parse expression: 1:9: parse error: unexpected end of input inside braces`)
	client.AssertExpectations(t)

	// Update rule fails
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(true)
	client.On("UpdateRule", testNID, sampleAlert1).Return(errors.New("error"))
	c, _ = buildContext(sampleAlert1, http.MethodPut, "/", v1alertPath, testNID)
	c.SetParamNames("file_prefix", ruleNameParam)
	c.SetParamValues(testNID, sampleAlert1.Alert)

	err = GetUpdateAlertHandler(client)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)

	// Reload Prometheus fails
	client = &mocks.PrometheusAlertClient{}
	client.On("RuleExists", testNID, sampleAlert1.Alert).Return(true)
	client.On("UpdateRule", testNID, sampleAlert1).Return(nil)
	client.On("ReloadPrometheus").Return(errors.New("error"))
	c, _ = buildContext(sampleAlert1, http.MethodPut, "/", v1alertPath, testNID)
	c.SetParamNames("file_prefix", ruleNameParam)
	c.SetParamValues(testNID, sampleAlert1.Alert)

	err = GetUpdateAlertHandler(client)(c)
	assert.Equal(t, http.StatusInternalServerError, err.(*echo.HTTPError).Code)
	assert.EqualError(t, err, `code=500, message=error`)
	client.AssertExpectations(t)
}

func TestGetBulkAlertUpdateHandler(t *testing.T) {
	// Successful Bulk Update
	client := &mocks.PrometheusAlertClient{}
	bulkAlerts := []rulefmt.Rule{sampleAlert1, sampleAlert2}
	sampleUpdateResult := alert.BulkUpdateResults{
		Errors:   map[string]error{},
		Statuses: map[string]string{"testAlert1": "created", "testAlert2": "created"},
	}
	client.On("BulkUpdateRules", testNID, bulkAlerts).Return(sampleUpdateResult, nil)
	client.On("ReloadPrometheus").Return(nil)

	c, rec := buildContext([]rulefmt.Rule{sampleAlert1, sampleAlert2}, http.MethodPut, "/", "/:file_prefix/alert/bulk", testNID)

	err := GetBulkAlertUpdateHandler(client)(c)
	assert.NoError(t, err)
	client.AssertExpectations(t)
	assert.Equal(t, http.StatusOK, rec.Code)

	var results alert.BulkUpdateResults
	err = json.Unmarshal(rec.Body.Bytes(), &results)
	assert.NoError(t, err)
	assert.Equal(t, sampleUpdateResult, results)

	// Bulk update with json format
	c, rec = buildContext([]alert.RuleJSONWrapper{sampleJSONRule1, sampleJSONRule2}, http.MethodPut, "/", "/:file_prefix/alert/bulk", testNID)
	err = GetBulkAlertUpdateHandler(client)(c)
	assert.NoError(t, err)
	client.AssertExpectations(t)
	assert.Equal(t, http.StatusOK, rec.Code)

	err = json.Unmarshal(rec.Body.Bytes(), &results)
	assert.NoError(t, err)
	assert.Equal(t, sampleUpdateResult, results)
}

type tenancyTestCase struct {
	name           string
	tenantProvider paramProvider
	context        *echo.Context
	expectedTenant string
	expectedError  error
}

func (tc *tenancyTestCase) RunTest(t *testing.T) {
	handler := func(c echo.Context) error { return nil }

	tenancyFunc := tenancyMiddlewareProvider(tc.tenantProvider)
	if tc.expectedError != nil {
		assert.EqualError(t, tenancyFunc(handler)(*tc.context), tc.expectedError.Error())
	} else {
		assert.NoError(t, tenancyFunc(handler)(*tc.context))
		assert.Equal(t, (*tc.context).Get(tenantIDParam), tc.expectedTenant)
	}
}

func TestTenancyMiddleware(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()

	plainReq := httptest.NewRequest(http.MethodGet, "/", nil)
	plainContext := e.NewContext(plainReq, rec)

	pathTenantContext := e.NewContext(plainReq, rec)
	pathTenantContext.SetParamNames(tenantIDParam)
	pathTenantContext.SetParamValues(testNID)

	mtClient := &mocks.PrometheusAlertClient{}
	mtClient.On("Tenancy").Return(&alert.TenancyConfig{RestrictorLabel: testNID})

	tests := []tenancyTestCase{{
		name:           "multi-tenant with path provided tenant",
		tenantProvider: pathTenantProvider,
		context:        &pathTenantContext,
		expectedTenant: testNID,
	}, {
		name:           "multi-tenant without path provided tenant",
		tenantProvider: pathTenantProvider,
		context:        &plainContext,
		expectedError:  errors.New("code=400, message=Must provide tenant_id parameter"),
	}}

	for _, test := range tests {
		t.Run(test.name, test.RunTest)
	}
}

func TestDecodeRulePostRequest(t *testing.T) {
	// Successful Decode
	c, _ := buildContext(sampleAlert1, http.MethodPost, "/", v1alertPath, testNID)
	conf, err := decodeRulePostRequest(c)
	assert.NoError(t, err)
	assert.Equal(t, sampleAlert1, conf)

	// Decode JSONWrapped Route
	c, _ = buildContext(sampleJSONRule1, http.MethodPost, "/", v1alertPath, testNID)
	conf, err = decodeRulePostRequest(c)
	assert.NoError(t, err)
	assert.Equal(t, sampleAlert1, conf)

	// error decoding route
	c, _ = buildContext(struct {
		Alert int `json:"alert"`
	}{0}, http.MethodPost, "/", v1alertPath, testNID)
	_, err = decodeRulePostRequest(c)
	assert.EqualError(t, err, `error unmarshalling payload: json: cannot unmarshal number into Go struct field RuleJSONWrapper.alert of type string`)
}

func TestRuleValidate(t *testing.T) {
	r := rulefmt.Rule{
		Alert: "test",
		Expr:  "test",
	}
	err := alert.ValidateRule(r)
	assert.NoError(t, err)

	r = rulefmt.Rule{
		Alert: "test",
		Expr:  `test{label="value"`,
	}
	err = alert.ValidateRule(r)
	assert.EqualError(t, err, "Rule Validation Error; could not parse expression: 1:19: parse error: unexpected end of input inside braces")

	r = rulefmt.Rule{
		Alert:  "test",
		Expr:   "test",
		Labels: map[string]string{"1label": "value"},
	}
	err = alert.ValidateRule(r)
	assert.EqualError(t, err, "Rule Validation Error; invalid label name: 1label")
}

func buildContext(body interface{}, method, target, path, tenantID string) (echo.Context, *httptest.ResponseRecorder) {
	bytes, _ := json.Marshal(body)
	req := httptest.NewRequest(method, target, strings.NewReader(string(bytes)))
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetPath(path)
	c.SetParamNames("file_prefix")
	c.SetParamValues(tenantID)
	c.Set(tenantIDParam, tenantID) // to emulate middleware
	return c, rec
}
