/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package main

import (
	"flag"
	"fmt"
	"strings"

	"prometheus-configmanager/alertmanager/client"
	"prometheus-configmanager/alertmanager/handlers"
	"prometheus-configmanager/fsclient"
	"prometheus-configmanager/prometheus/alert"

	"github.com/golang/glog"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

const (
	defaultPort                   = "9101"
	defaultAlertmanagerURL        = "alertmanager:9093"
	defaultAlertmanagerConfigPath = "./alertmanager.yml"
	defaultTemplateDir            = "./templates/"
)

func main() {
	port := flag.String("port", defaultPort, fmt.Sprintf("Port to listen for requests. Default is %s", defaultPort))
	alertmanagerConfPath := flag.String("alertmanager-conf", defaultAlertmanagerConfigPath, fmt.Sprintf("Path to alertmanager configuration file. Default is %s", defaultAlertmanagerConfigPath))
	alertmanagerURL := flag.String("alertmanagerURL", defaultAlertmanagerURL, fmt.Sprintf("URL of the alertmanager instance that is being used. Default is %s", defaultAlertmanagerURL))
	matcherLabel := flag.String("multitenant-label", "", fmt.Sprintf("LabelName to use for enabling multitenancy through route matching. Leave empty for single tenant use cases."))
	templateDirPath := flag.String("template-directory", defaultTemplateDir, fmt.Sprintf("Directory where template files are stored. Default is %s", defaultTemplateDir))
	flag.Parse()

	if !strings.HasSuffix(*templateDirPath, "/") {
		*templateDirPath += "/"
	}

	tenancy := &alert.TenancyConfig{
		RestrictorLabel: *matcherLabel,
	}

	e := echo.New()
	e.Use(middleware.CORS())

	fileLocks, err := alert.NewFileLocker(alert.NewDirectoryClient(*templateDirPath))
	if err != nil {
		panic(fmt.Errorf("error configuring file configmanager: %v", err))
	}

	receiverClient := client.NewClient(*alertmanagerConfPath, *alertmanagerURL, tenancy, fsclient.NewFSClient("/"))
	templateClient := client.NewTemplateClient(fsclient.NewFSClient(*templateDirPath), fileLocks)

	handlers.RegisterBaseHandlers(e)
	handlers.RegisterV0Handlers(e, receiverClient)
	handlers.RegisterV1Handlers(e, receiverClient, templateClient)

	glog.Infof("Alertmanager Config server listening on port: %s\n", *port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", *port)))
}
