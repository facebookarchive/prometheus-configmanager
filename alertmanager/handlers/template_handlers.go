/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package handlers

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"prometheus-configmanager/alertmanager/client"

	"github.com/labstack/echo"
)

func GetGetTemplateFileHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)
		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("error getting file %s: file does not exist", filename).Error())
		}

		file, err := tmplClient.GetTemplateFile(filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error getting template file: %v", err))
		}
		return c.JSON(http.StatusOK, file)
	}
}

func GetPostTemplateFileHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("file %s already exists", filename))
		}

		body, err := readStringBody(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}

		err = tmplClient.CreateTemplateFile(filename, body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error creating template file: %v", err))
		}

		err = amClient.AddTemplateFile(getFullFilePath(filename, tmplClient))
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error creating template file: %v", err))
		}

		return c.String(http.StatusOK, "Created")
	}
}

func GetPutTemplateFileHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error editing file %s: file does not exist", filename))
		}

		body, err := readStringBody(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}

		err = tmplClient.EditTemplateFile(filename, body)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error creating template file: %v", err))
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetDeleteTemplateFileHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error deleting file: file %s does not exist", filename))
		}

		err = tmplClient.DeleteTemplateFile(filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error deleting template file: %v", err))
		}

		err = amClient.RemoveTemplateFile(getFullFilePath(filename, tmplClient))
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error deleting template file: %v", err))
		}

		return c.NoContent(http.StatusOK)
	}
}

func GetGetTemplatesHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error getting file: file %s does not exist", filename))
		}

		tmps, err := tmplClient.GetTemplates(filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error getting templates: %s", err.Error()))
		}
		return c.JSON(http.StatusOK, tmps)
	}
}

func GetGetTemplateHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)
		tmplName := c.Get(templateNameParam).(string)

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error getting template: file %s does not exist", filename))
		}

		tmpl, err := tmplClient.GetTemplate(filename, tmplName)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error getting template: %s", err.Error()))
		}
		return c.JSON(http.StatusOK, tmpl)
	}
}

func GetPostTemplateHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)
		tmplName := c.Get(templateNameParam).(string)

		tmplText, err := readStringBody(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error getting template: file %s does not exist", filename))
		}

		err = tmplClient.AddTemplate(filename, tmplName, tmplText)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error adding template: %s", err.Error()))
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetPutTemplateHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)
		tmplName := c.Get(templateNameParam).(string)

		tmplText, err := readStringBody(c)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error getting template: file %s does not exist", filename))
		}

		err = tmplClient.EditTemplate(filename, tmplName, tmplText)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error editing template: %s", err.Error()))
		}
		return c.NoContent(http.StatusOK)
	}
}

func GetDeleteTemplateHandler(amClient client.AlertmanagerClient, tmplClient client.TemplateClient) func(c echo.Context) error {
	return func(c echo.Context) error {
		filename := c.Get(templateFilenameParam).(string)
		tmplName := c.Get(templateNameParam).(string)

		exists, err := fileExists(amClient, tmplClient, filename)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		if !exists {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error getting template: file %s does not exist", filename))
		}

		err = tmplClient.DeleteTemplate(filename, tmplName)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error deleting template: %s", err.Error()))
		}
		return c.NoContent(http.StatusOK)
	}
}

func stringParamProvider(paramName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestedParam := c.Param(paramName)
			if requestedParam == "" {
				return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Must provide %s parameter", paramName))
			}
			c.Set(paramName, requestedParam)
			return next(c)
		}
	}
}

func fileExists(amClient client.AlertmanagerClient, tmplClient client.TemplateClient, filename string) (bool, error) {
	files, err := amClient.GetTemplateFileList()
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if file == getFullFilePath(filename, tmplClient) {
			return true, nil
		}
	}
	return false, nil
}

func getFullFilePath(filename string, tmplClient client.TemplateClient) string {
	return tmplClient.Root() + filename + client.TemplateFilePostfix
}

func readStringBody(c echo.Context) (string, error) {
	body, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return string(body), fmt.Errorf("error reading request body: %v", err)
	}
	return string(body), nil
}
