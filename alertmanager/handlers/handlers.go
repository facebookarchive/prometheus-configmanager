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

	"prometheus-configmanager/alertmanager/client"
	"prometheus-configmanager/alertmanager/config"

	"github.com/labstack/echo"
)

const (
	v0rootPath               = "/:tenant_id"
	v0receiverPath           = "/receiver"
	v0RoutePath              = "/receiver/route"
	v0receiverNameQueryParam = "receiver"

	v1rootPath       = "/v1"
	tenantIDPart     = "/:tenant_id"
	v1TenantRootPath = v1rootPath + tenantIDPart

	v1receiverPath     = "/receiver"
	v1receiverNamePath = v1receiverPath + "/:" + receiverNameParam
	v1routePath        = "/route"
	v1GlobalPath       = "/global"
	v1TenantPath       = "/tenants"
	v1TenancyPath      = "/tenancy"

	receiverNameParam = "receiver_name"
	tenantIDParam     = "tenant_id"
)

func RegisterBaseHandlers(e *echo.Echo) {
	e.GET("/", statusHandler)
}

func RegisterV0Handlers(e *echo.Echo, client client.AlertmanagerClient) {
	v0 := e.Group(v0rootPath)
	v0.Use(tenancyMiddlewareProvider(client, pathTenantProvider))

	v0.POST(v0receiverPath, GetReceiverPostHandler(client))
	v0.GET(v0receiverPath, GetGetReceiversHandler(client))
	v0.DELETE(v0receiverPath, GetDeleteReceiverHandler(client, v0receiverNameQueryProvider))
	v0.PUT(v0receiverPath+"/:"+receiverNameParam, GetUpdateReceiverHandler(client, receiverNamePathProvider))

	v0.POST(v0RoutePath, GetUpdateRouteHandler(client))
	v0.GET(v0RoutePath, GetGetRouteHandler(client))
}

func RegisterV1Handlers(e *echo.Echo, client client.AlertmanagerClient) {
	v1 := e.Group(v1rootPath)
	v1Tenant := e.Group(v1TenantRootPath)

	// these don't require tenancy so register before middleware
	v1.GET(v1TenantPath, GetGetTenantsHandler(client))
	v1.GET(v1TenancyPath, GetGetTenancyHandler(client))

	// TODO: Remove the tenant param from these paths
	v1Tenant.POST(v1GlobalPath, GetUpdateGlobalConfigHandler(client))
	v1Tenant.GET(v1GlobalPath, GetGetGlobalConfigHandler(client))

	v1Tenant.Use(tenancyMiddlewareProvider(client, pathTenantProvider))

	v1Tenant.POST(v1receiverPath, GetReceiverPostHandler(client))
	v1Tenant.GET(v1receiverPath, GetGetReceiversHandler(client))

	v1Tenant.DELETE(v1receiverNamePath, GetDeleteReceiverHandler(client, receiverNamePathProvider))
	v1Tenant.PUT(v1receiverNamePath, GetUpdateReceiverHandler(client, receiverNamePathProvider))
	v1Tenant.GET(v1receiverNamePath, GetGetReceiversHandler(client))

	v1Tenant.POST(v1routePath, GetUpdateRouteHandler(client))
	v1Tenant.GET(v1routePath, GetGetRouteHandler(client))

}

func statusHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Alertmanager Config server")
}

type paramProvider func(c echo.Context) string

// For v0 tenant_id field in path
var pathTenantProvider = func(c echo.Context) string {
	return c.Param(tenantIDParam)
}

var v0receiverNameQueryProvider = func(c echo.Context) string {
	return c.QueryParam(v0receiverNameQueryParam)
}

var receiverNamePathProvider = func(c echo.Context) string {
	return c.Param(receiverNameParam)
}

// Returns middleware func to check for tenant_id dependent on tenancy of the client
func tenancyMiddlewareProvider(client client.AlertmanagerClient, getTenantID paramProvider) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			providedTenantID := getTenantID(c)
			if client.Tenancy() != nil && providedTenantID == "" {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Must provide %s parameter", tenantIDParam))
			}
			c.Set(tenantIDParam, providedTenantID)
			return next(c)
		}
	}
}

// GetReceiverPostHandler returns a handler function that creates a new
// receiver and then reloads alertmanager
func GetReceiverPostHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		receiver, err := decodeReceiverPostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		tenantID := c.Get(tenantIDParam).(string)

		err = client.CreateReceiver(tenantID, receiver)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ReloadAlertmanager()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusOK)
	}
}

// GetGetReceiversHandler returns a handler function to retrieve receivers for
// a filePrefix
func GetGetReceiversHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenantID := c.Get(tenantIDParam).(string)
		receiverName := c.Param(receiverNameParam)

		recs, err := client.GetReceivers(tenantID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		if receiverName != "" {
			for _, rec := range recs {
				if rec.Name == receiverName {
					return c.JSON(http.StatusOK, rec)
				}
			}
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("Receiver %s not found", receiverName))
		}
		return c.JSON(http.StatusOK, recs)
	}
}

func GetGetTenantsHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenants, err := client.GetTenants()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, tenants)
	}
}

func GetGetTenancyHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, client.Tenancy())
	}
}

// GetUpdateReceiverHandler returns a handler function to update a receivers
func GetUpdateReceiverHandler(client client.AlertmanagerClient, getReceiverName paramProvider) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenantID := c.Get(tenantIDParam).(string)
		receiverName := getReceiverName(c)

		newReceiver, err := decodeReceiverPostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.UpdateReceiver(tenantID, receiverName, &newReceiver)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ReloadAlertmanager()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetDeleteReceiverHandler(client client.AlertmanagerClient, getReceiverName paramProvider) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenantID := c.Get(tenantIDParam).(string)

		err := client.DeleteReceiver(tenantID, getReceiverName(c))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ReloadAlertmanager()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetGetRouteHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenantID := c.Get(tenantIDParam).(string)

		route, err := client.GetRoute(tenantID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		return c.JSON(http.StatusOK, *route)
	}
}

func GetUpdateRouteHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		tenantID := c.Get(tenantIDParam).(string)

		newRoute, err := decodeRoutePostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		err = client.ModifyTenantRoute(tenantID, &newRoute)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ReloadAlertmanager()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetUpdateGlobalConfigHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		newGlobalConfig, err := decodeGlobalConfigPostRequest(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		err = client.SetGlobalConfig(newGlobalConfig)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		err = client.ReloadAlertmanager()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetGetGlobalConfigHandler(client client.AlertmanagerClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		globalConf, err := client.GetGlobalConfig()
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, globalConf)
	}
}

func decodeGlobalConfigPostRequest(c echo.Context) (config.GlobalConfig, error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return config.GlobalConfig{}, fmt.Errorf("error reading request body: %v", err)
	}
	globalConfig := config.GlobalConfig{}
	err = json.Unmarshal(body, &globalConfig)
	if err != nil {
		return config.GlobalConfig{}, fmt.Errorf("error unmarshalling payload: %v", err)
	}
	return globalConfig, nil
}

func decodeReceiverPostRequest(c echo.Context) (config.Receiver, error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return config.Receiver{}, fmt.Errorf("error reading request body: %v", err)
	}
	receiver := config.Receiver{}
	err = json.Unmarshal(body, &receiver)
	if err == nil {
		return receiver, nil
	}

	// Try to unmarshal into the ReceiverJSONWrapper struct if prometheus struct doesn't work
	jsonPayload := config.ReceiverJSONWrapper{}
	err = json.Unmarshal(body, &jsonPayload)
	if err != nil {
		return receiver, fmt.Errorf("error unmarshalling payload: %v", err)
	}

	return jsonPayload.ToReceiverFmt()
}

func decodeRoutePostRequest(c echo.Context) (config.Route, error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return config.Route{}, fmt.Errorf("error reading request body: %v", err)
	}
	route := config.Route{}
	err = json.Unmarshal(body, &route)
	if err != nil {
		return config.Route{}, fmt.Errorf("error unmarshalling route: %v", err)
	}
	return route, nil
}
