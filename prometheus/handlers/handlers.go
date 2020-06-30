/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"prometheus-configmanager/prometheus/alert"

	"github.com/labstack/echo"
	"github.com/prometheus/prometheus/pkg/rulefmt"
)

const (
	v0rootPath        = "/:tenant_id"
	v0alertPath       = "/alert"
	v0alertUpdatePath = v0alertPath + "/:" + ruleNameParam
	v0alertBulkPath   = v0alertPath + "/bulk"

	ruleNameParam = "alert_name"

	tenantIDParam = "tenant_id"

	v1rootPath       = "/v1"
	v1TenantRootPath = v1rootPath + "/:tenant_id"

	v1alertPath     = "/alert"
	v1alertBulkPath = v1alertPath + "/bulk"
	v1alertNamePath = v1alertPath + "/:" + ruleNameParam
	v1TenancyPath   = "/tenancy"
)

func statusHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Prometheus Config server")
}

func RegisterBaseHandlers(e *echo.Echo) {
	e.GET("/", statusHandler)
}

func RegisterV0Handlers(e *echo.Echo, alertClient alert.PrometheusAlertClient) {
	v0 := e.Group(v0rootPath)
	v0.Use(tenancyMiddlewareProvider(pathTenantProvider))

	v0.POST(v0alertPath, GetConfigureAlertHandler(alertClient))
	v0.GET(v0alertPath, GetRetrieveAlertHandler(alertClient))
	v0.DELETE(v0alertPath, GetDeleteAlertHandler(alertClient, queryAlertNameProvider))

	v0.PUT(v0alertUpdatePath, GetUpdateAlertHandler(alertClient))

	v0.PUT(v0alertBulkPath, GetBulkAlertUpdateHandler(alertClient))
}

func RegisterV1Handlers(e *echo.Echo, alertClient alert.PrometheusAlertClient) {
	v1 := e.Group(v1rootPath)

	v1.GET(v1TenancyPath, GetGetTenancyHandler(alertClient))

	v1Tenant := e.Group(v1TenantRootPath)
	v1Tenant.Use(tenancyMiddlewareProvider(pathTenantProvider))

	v1Tenant.POST(v1alertPath, GetConfigureAlertHandler(alertClient))
	v1Tenant.GET(v1alertPath, GetRetrieveAlertHandler(alertClient))

	v1Tenant.DELETE(v1alertNamePath, GetDeleteAlertHandler(alertClient, pathAlertNameProvider))
	v1Tenant.PUT(v1alertNamePath, GetUpdateAlertHandler(alertClient))
	v1Tenant.GET(v1alertNamePath, GetRetrieveAlertHandler(alertClient))

	v1Tenant.POST(v1alertBulkPath, GetBulkAlertUpdateHandler(alertClient))
}

// Returns middleware func to check for tenant_id
func tenancyMiddlewareProvider(getTenantID paramProvider) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			providedTenantID := getTenantID(c)
			if providedTenantID == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "Must provide tenant_id parameter")
			}
			c.Set(tenantIDParam, providedTenantID)
			return next(c)
		}
	}
}

type paramProvider func(c echo.Context) string

// V0 tenantID is a path parameter
var pathTenantProvider = func(c echo.Context) string {
	return c.Param(tenantIDParam)
}

var pathAlertNameProvider = func(c echo.Context) string {
	return c.Param(ruleNameParam)
}

var queryAlertNameProvider = func(c echo.Context) string {
	return c.QueryParam(ruleNameParam)
}

// GetConfigureAlertHandler returns a handler that calls the client method WriteAlert() to
// write the alert configuration from the body of this request
func GetConfigureAlertHandler(client alert.PrometheusAlertClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		rule, err := decodeRulePostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		tenantID := c.Get(tenantIDParam).(string)

		err = client.ValidateRule(rule)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		if client.RuleExists(tenantID, rule.Alert) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Rule '%s' already exists", rule.Alert))
		}

		err = client.WriteRule(tenantID, rule)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		err = client.ReloadPrometheus()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetRetrieveAlertHandler(client alert.PrometheusAlertClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		ruleName := c.QueryParam(ruleNameParam)
		tenantID := c.Get(tenantIDParam).(string)

		rules, err := client.ReadRules(tenantID, ruleName)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		jsonRules, err := rulesToJSON(rules)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, jsonRules)
	}
}

func GetDeleteAlertHandler(client alert.PrometheusAlertClient, getRuleName paramProvider) func(c echo.Context) error {
	return func(c echo.Context) error {
		ruleName := getRuleName(c)
		tenantID := c.Get(tenantIDParam).(string)

		if ruleName == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "No rule name provided")
		}
		err := client.DeleteRule(tenantID, ruleName)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		err = client.ReloadPrometheus()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusNoContent, fmt.Sprintf("rule %s deleted", ruleName))
	}
}

func GetUpdateAlertHandler(client alert.PrometheusAlertClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		ruleName := c.Param(ruleNameParam)
		tenantID := c.Get(tenantIDParam).(string)

		if ruleName == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "No rule name provided")
		}

		if !client.RuleExists(tenantID, ruleName) {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Rule '%s' does not exist", ruleName))
		}

		rule, err := decodeRulePostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ValidateRule(rule)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.UpdateRule(tenantID, rule)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		err = client.ReloadPrometheus()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusNoContent)
	}
}

func GetBulkAlertUpdateHandler(client alert.PrometheusAlertClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenantID := c.Get(tenantIDParam).(string)

		rules, err := decodeBulkRulesPostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		for _, rule := range rules {
			err = client.ValidateRule(rule)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			}
		}

		results, err := client.BulkUpdateRules(tenantID, rules)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ReloadPrometheus()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, results)
	}
}

func GetGetTenancyHandler(client alert.PrometheusAlertClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, client.Tenancy())
	}
}

func decodeRulePostRequest(c echo.Context) (rulefmt.Rule, error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return rulefmt.Rule{}, fmt.Errorf("error reading request body: %v", err)
	}
	// First try unmarshaling into prometheus rulefmt.Rule{}
	payload := rulefmt.Rule{}
	err = json.Unmarshal(body, &payload)
	if err == nil {
		return payload, nil
	}
	// Try to unmarshal into the RuleJSONWrapper struct if prometheus struct doesn't work
	jsonPayload := alert.RuleJSONWrapper{}
	err = json.Unmarshal(body, &jsonPayload)
	if err != nil {
		return payload, fmt.Errorf("error unmarshalling payload: %v", err)
	}
	return jsonPayload.ToRuleFmt()
}

func decodeBulkRulesPostRequest(c echo.Context) ([]rulefmt.Rule, error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return []rulefmt.Rule{}, fmt.Errorf("error reading request body: %v", err)
	}
	var payload []rulefmt.Rule
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return payload, fmt.Errorf("error unmarshalling payload: %v", err)
	}
	return payload, nil
}

func rulesToJSON(rules []rulefmt.Rule) ([]alert.RuleJSONWrapper, error) {
	ret := make([]alert.RuleJSONWrapper, 0)

	for _, rule := range rules {
		jsonRule, err := rulefmtToJSON(rule)
		if err != nil {
			return ret, err
		}
		ret = append(ret, *jsonRule)
	}
	return ret, nil
}

func rulefmtToJSON(rule rulefmt.Rule) (*alert.RuleJSONWrapper, error) {
	duration, err := time.ParseDuration(rule.For.String())
	if err != nil {
		return nil, err
	}
	return &alert.RuleJSONWrapper{
		Record:      rule.Record,
		Alert:       rule.Alert,
		Expr:        rule.Expr,
		For:         duration.String(),
		Labels:      rule.Labels,
		Annotations: rule.Annotations,
	}, nil
}
