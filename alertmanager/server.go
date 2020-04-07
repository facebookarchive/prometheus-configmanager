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

	"prometheus-configmanager/alertmanager/client"
	"prometheus-configmanager/alertmanager/handlers"
	"prometheus-configmanager/fsclient"
	"prometheus-configmanager/prometheus/alert"

	"github.com/golang/glog"
	"github.com/labstack/echo"
)

const (
	defaultPort                   = "9101"
	defaultAlertmanagerURL        = "alertmanager:9093"
	defaultAlertmanagerConfigPath = "./alertmanager.yml"
)

func main() {
	port := flag.String("port", defaultPort, fmt.Sprintf("Port to listen for requests. Default is %s", defaultPort))
	alertmanagerConfPath := flag.String("alertmanager-conf", defaultAlertmanagerConfigPath, fmt.Sprintf("Path to alertmanager configuration file. Default is %s", defaultAlertmanagerConfigPath))
	alertmanagerURL := flag.String("alertmanagerURL", defaultAlertmanagerURL, fmt.Sprintf("URL of the alertmanager instance that is being used. Default is %s", defaultAlertmanagerURL))
	matcherLabel := flag.String("multitenant-label", "", fmt.Sprintf("LabelName to use for enabling multitenancy through route matching. Leave empty for single tenant use cases."))
	flag.Parse()

	tenancy := &alert.TenancyConfig{
		RestrictorLabel: *matcherLabel,
	}

	e := echo.New()

	receiverClient := client.NewClient(*alertmanagerConfPath, *alertmanagerURL, tenancy, fsclient.NewFSClient())

	handlers.RegisterBaseHandlers(e)
	handlers.RegisterV0Handlers(e, receiverClient)
	handlers.RegisterV1Handlers(e, receiverClient)

	glog.Infof("Alertmanager Config server listening on port: %s\n", *port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", *port)))

}
