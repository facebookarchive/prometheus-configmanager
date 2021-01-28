/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/facebookincubator/prometheus-configmanager/fsclient"
	"github.com/facebookincubator/prometheus-configmanager/prometheus/alert"

	"github.com/facebookincubator/prometheus-configmanager/alertmanager/config"

	"gopkg.in/yaml.v2"
)

type AlertmanagerClient interface {
	CreateReceiver(tenantID string, rec config.Receiver) error
	GetReceivers(tenantID string) ([]config.Receiver, error)
	UpdateReceiver(tenantID, receiverName string, newRec *config.Receiver) error
	DeleteReceiver(tenantID, receiverName string) error

	// ModifyNetworkRoute updates an existing routing tree for the given
	// tenant, or creates one if it already exists. Ensures that the base
	// route matches all alerts with label "tenantID" = <tenantID>.
	ModifyTenantRoute(tenantID string, route *config.Route) error

	// GetRoute returns the routing tree for the given tenantID
	GetRoute(tenantID string) (*config.Route, error)

	// GetTenants returns a list of tenants configured in the system
	GetTenants() ([]string, error)

	GetGlobalConfig() (*config.GlobalConfig, error)
	SetGlobalConfig(globalConfig config.GlobalConfig) error

	GetTemplateFileList() ([]string, error)
	AddTemplateFile(path string) error
	RemoveTemplateFile(path string) error

	// ReloadAlertmanager triggers the alertmanager process to reload the
	// configuration file(s)
	ReloadAlertmanager() error

	Tenancy() *alert.TenancyConfig
}

type ClientConfig struct {
	ConfigPath      string
	AlertmanagerURL string
	FsClient        fsclient.FSClient
	Tenancy         *alert.TenancyConfig
	DeleteRoutes    bool
}

// Client provides methods to create and read receiver configurations
type client struct {
	conf ClientConfig
	sync.RWMutex
}

func NewClient(conf ClientConfig) AlertmanagerClient {
	return &client{
		conf: ClientConfig{
			ConfigPath:      conf.ConfigPath,
			AlertmanagerURL: conf.AlertmanagerURL,
			FsClient:        conf.FsClient,
			Tenancy:         conf.Tenancy,
			DeleteRoutes:    conf.DeleteRoutes,
		},
	}
}

// CreateReceiver writes a new receiver to the config file with the tenantID
// prepended to the name so multiple tenants can be supported
func (c *client) CreateReceiver(tenantID string, rec config.Receiver) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	rec.Secure(tenantID)

	conf.Receivers = append(conf.Receivers, &rec)
	err = conf.Validate()
	if err != nil {
		return err
	}
	return c.writeConfigFile(conf)
}

// GetReceivers returns the receiver configs for the given tenantID
func (c *client) GetReceivers(tenantID string) ([]config.Receiver, error) {
	c.RLock()
	defer c.RUnlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return []config.Receiver{}, nil
	}

	recs := make([]config.Receiver, 0)
	for _, rec := range conf.Receivers {
		if strings.HasPrefix(rec.Name, config.ReceiverTenantPrefix(tenantID)) {
			if rec.Name == config.ReceiverTenantPrefix(tenantID)+config.TenantBaseRoutePostfix {
				continue
			}
			rec.Unsecure(tenantID)
			recs = append(recs, *rec)
		}
	}
	return recs, nil
}

// UpdateReceiver modifies an existing receiver
func (c *client) UpdateReceiver(tenantID, receiverName string, newRec *config.Receiver) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	newRec.Secure(tenantID)

	receiverToUpdate := config.SecureReceiverName(receiverName, tenantID)
	receiverIdx := -1
	for idx, rec := range conf.Receivers {
		if rec.Name == receiverToUpdate {
			receiverIdx = idx
			break
		}
	}
	if receiverIdx < 0 {
		return fmt.Errorf("Receiver '%s' not found", newRec.Name)
	}

	conf.Receivers[receiverIdx] = newRec
	err = conf.Validate()
	if err != nil {
		return fmt.Errorf("Error updating receiver: %v", err)
	}
	return c.writeConfigFile(conf)
}

// DeleteReceiver removes a receiver from the configuration
func (c *client) DeleteReceiver(tenantID, receiverName string) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	receiverToDelete := config.SecureReceiverName(receiverName, tenantID)

	foundReceiver := false
	for idx, rec := range conf.Receivers {
		if rec.Name == receiverToDelete {
			conf.Receivers = append(conf.Receivers[:idx], conf.Receivers[idx+1:]...)
			foundReceiver = true
			break
		}
	}
	if !foundReceiver {
		return fmt.Errorf("receiver '%s' does not exist", receiverName)
	}

	if c.conf.DeleteRoutes {
		conf.RemoveReceiverFromRoute(receiverToDelete)
	} else {
		if conf.SearchRoutesForReceiver(receiverToDelete) {
			return fmt.Errorf("reciever '%s' referenced in route. Update routing tree and remove references before deleting this receiver", receiverName)
		}
	}

	return c.writeConfigFile(conf)
}

// ModifyTenantRoute takes a new route for a tenant and replaces the old one,
// ensuring that receivers are properly named and the resulting config is valid.
// Creates a new one if it doesn't already exist. If single-tenant client this
// just modifies the entire routing tree
func (c *client) ModifyTenantRoute(tenantID string, route *config.Route) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	// ensure base route is valid base route for this tenant
	baseRoute := c.getBaseRouteForTenant(tenantID, conf)
	if route.Receiver != baseRoute.Receiver {
		return fmt.Errorf("route base receiver is incorrect (should be \"%s\"). "+
			"The base node should match nothing, then add routes as children of the base node", baseRoute.Receiver)
	}

	if route.Match == nil {
		route.Match = map[string]string{}
	}

	if c.conf.Tenancy.RestrictorLabel != "" {
		route.Match[c.conf.Tenancy.RestrictorLabel] = tenantID
	}

	for _, childRoute := range route.Routes {
		if childRoute == nil {
			continue
		}
		secureRoute(tenantID, childRoute)
	}

	tenantRouteIdx := conf.GetRouteIdx(config.MakeBaseRouteName(tenantID))
	if tenantRouteIdx < 0 {
		err := conf.InitializeNetworkBaseRoute(route, c.conf.Tenancy.RestrictorLabel, tenantID)
		if err != nil {
			return err
		}
	} else {
		conf.Route.Routes[tenantRouteIdx] = route
	}

	err = conf.Validate()
	if err != nil {
		return err
	}
	return c.writeConfigFile(conf)
}

// GetRoute returns the base route for the given tenantID
func (c *client) GetRoute(tenantID string) (*config.Route, error) {
	c.RLock()
	defer c.RUnlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return &config.Route{}, err
	}

	routeIdx := conf.GetRouteIdx(config.MakeBaseRouteName(tenantID))
	if routeIdx >= 0 {
		route := conf.Route.Routes[routeIdx]
		unsecureRoute(tenantID, route)
		return route, nil
	}
	return nil, fmt.Errorf("Route for tenant %s does not exist", tenantID)
}

func (c *client) GetTenants() ([]string, error) {
	c.RLock()
	defer c.RUnlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return []string{}, err
	}

	tenants := make([]string, 0)
	for _, rec := range conf.Receivers {
		if strings.Contains(rec.Name, config.TenantBaseRoutePostfix) {
			tenants = append(tenants, rec.Name[0:strings.Index(rec.Name, config.TenantBaseRoutePostfix)-1])
		}
	}
	return tenants, nil
}

func (c *client) GetTemplateFileList() ([]string, error) {
	c.RLock()
	defer c.RUnlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return []string{}, err
	}

	return conf.Templates, nil
}

func (c *client) AddTemplateFile(path string) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	conf.Templates = append(conf.Templates, path)

	return c.writeConfigFile(conf)
}

func (c *client) RemoveTemplateFile(path string) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	tmplIdx := -1
	for idx, tmpl := range conf.Templates {
		if tmpl == path {
			tmplIdx = idx
			break
		}
	}
	if tmplIdx == -1 {
		return fmt.Errorf("path not found: %s", path)
	}
	// Remove element from template list
	conf.Templates = append(conf.Templates[:tmplIdx], conf.Templates[tmplIdx+1:]...)

	return c.writeConfigFile(conf)
}

func (c *client) ReloadAlertmanager() error {
	resp, err := http.Post(fmt.Sprintf("http://%s%s", c.conf.AlertmanagerURL, "/-/reload"), "text/plain", &bytes.Buffer{})
	if err != nil {
		return fmt.Errorf("error reloading alertmanager: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("code: %d error reloading alertmanager: %s", resp.StatusCode, msg)
	}
	return nil
}

func (c *client) GetGlobalConfig() (*config.GlobalConfig, error) {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return nil, err
	}

	return conf.Global, nil
}

func (c *client) SetGlobalConfig(globalConfig config.GlobalConfig) error {
	c.Lock()
	defer c.Unlock()
	conf, err := c.readConfigFile()
	if err != nil {
		return err
	}

	conf.Global = &globalConfig
	err = conf.Validate()
	if err != nil {
		return err
	}

	return c.writeConfigFile(conf)
}

func (c *client) Tenancy() *alert.TenancyConfig {
	return c.conf.Tenancy
}

func (c *client) readConfigFile() (*config.Config, error) {
	configFile := config.Config{}
	file, err := c.conf.FsClient.ReadFile(c.conf.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config files: %v", err)
	}
	err = yaml.Unmarshal(file, &configFile)

	return &configFile, err
}

func (c *client) writeConfigFile(conf *config.Config) error {
	yamlFile, err := yaml.Marshal(conf)
	if err != nil {
		return fmt.Errorf("error marshaling config file: %v", err)
	}
	err = c.conf.FsClient.WriteFile(c.conf.ConfigPath, yamlFile, 0660)
	if err != nil {
		return fmt.Errorf("error writing config file: %v", err)
	}
	return nil
}

// secureRoute ensure that all receivers in the route have the
// proper tenantID-prefixed receiver name
func secureRoute(tenantID string, route *config.Route) {
	route.Receiver = config.SecureReceiverName(route.Receiver, tenantID)
	for _, childRoute := range route.Routes {
		secureRoute(tenantID, childRoute)
	}
}

// unsecureRoute traverses a routing tree and reverts receiver
// names to their non-prefixed original names
func unsecureRoute(tenantID string, route *config.Route) {
	if !strings.HasSuffix(route.Receiver, config.TenantBaseRoutePostfix) {
		route.Receiver = config.UnsecureReceiverName(route.Receiver, tenantID)
	}
	for _, childRoute := range route.Routes {
		unsecureRoute(tenantID, childRoute)
	}
}

func (c *client) getBaseRouteForTenant(tenantID string, conf *config.Config) *config.Route {
	baseRouteName := config.MakeBaseRouteName(tenantID)
	for _, route := range conf.Route.Routes {
		if route.Receiver == baseRouteName {
			return route
		}
	}
	return &config.Route{Receiver: config.MakeBaseRouteName(tenantID)}
}
