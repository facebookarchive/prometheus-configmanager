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
	"io/ioutil"
	"os"
	"strings"

	"github.com/facebookincubator/prometheus-configmanager/fsclient"
	"github.com/facebookincubator/prometheus-configmanager/prometheus/alert"
	"github.com/facebookincubator/prometheus-configmanager/prometheus/handlers"

	"github.com/golang/glog"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

const (
	defaultPort          = "9100"
	defaultPrometheusURL = "prometheus:9090"
	defaultTenancyLabel  = "tenant"
)

func main() {
	port := flag.String("port", defaultPort, fmt.Sprintf("Port to listen for requests. Default is %s", defaultPort))
	rulesDir := flag.String("rules-dir", ".", "Directory to write rules files. Default is '.'")
	prometheusURL := flag.String("prometheusURL", defaultPrometheusURL, fmt.Sprintf("URL of the prometheus instance that is reading these rules. Default is %s", defaultPrometheusURL))
	multitenancyLabel := flag.String("multitenant-label", "tenant", fmt.Sprintf("The label name to segment alerting rules to enable multi-tenant support, having each tenant's alerts in a separate file. Default is %s", defaultTenancyLabel))
	restrictQueries := flag.Bool("restrict-queries", false, "If this flag is set all alert rule expressions will be restricted to only match series with {<multitenant-label>=<tenant>}")
	flag.Parse()

	if !strings.HasSuffix(*rulesDir, "/") {
		*rulesDir += "/"
	}

	// Check if rulesDir exists and create it if not
	if _, err := os.Stat(*rulesDir); os.IsNotExist(err) {
		files, err := ioutil.ReadDir("/")
		if err != nil {
			glog.Fatalf("Could not stat directory: %v", err)
		}
		fmt.Println(files)
		err = os.Mkdir(*rulesDir, 0644)
		if err != nil {
			glog.Fatalf("Could not create rules directory: %v", err)
		}
	}

	fileLocks, err := alert.NewFileLocker(alert.NewDirectoryClient(*rulesDir))
	clientTenancy := alert.TenancyConfig{
		RestrictQueries: *restrictQueries,
		RestrictorLabel: *multitenancyLabel,
	}
	alertClient := alert.NewClient(fileLocks, *prometheusURL, fsclient.NewFSClient(*rulesDir), clientTenancy)
	if err != nil {
		glog.Fatalf("error creating alert client: %v", err)
	}

	e := echo.New()
	e.Use(middleware.CORS())
	e.Use(middleware.Logger())

	handlers.RegisterBaseHandlers(e)
	handlers.RegisterV0Handlers(e, alertClient)
	handlers.RegisterV1Handlers(e, alertClient)

	glog.Infof("Prometheus Config server listening on port: %s\n", *port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", *port)))
}
