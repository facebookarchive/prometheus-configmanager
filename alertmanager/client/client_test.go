/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package client

import (
	"regexp"
	"testing"

	"gopkg.in/yaml.v2"

	"prometheus-configmanager/alertmanager/config"
	tc "prometheus-configmanager/alertmanager/testcommon"
	"prometheus-configmanager/fsclient/mocks"
	"prometheus-configmanager/prometheus/alert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testNID              = "test"
	otherNID             = "other"
	testAlertmanagerFile = `global:
  resolve_timeout: 5m
  http_config: {}
  smtp_hello: localhost
  smtp_require_tls: true
  pagerduty_url: https://events.pagerduty.com/v2/enqueue
  hipchat_api_url: https://api.hipchat.com/
  opsgenie_api_url: https://api.opsgenie.com/
  wechat_api_url: https://qyapi.weixin.qq.com/cgi-bin/
  victorops_api_url: https://alert.victorops.com/integrations/generic/20131114/alert/
route:
  receiver: null_receiver
  group_by:
  - alertname
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  routes:
  - receiver: other_tenant_base_route
    match:
      tenantID: other
receivers:
- name: null_receiver
- name: test_receiver
- name: receiver
- name: other_tenant_base_route
- name: sample_tenant_base_route
- name: test_slack
  slack_configs:
  - api_url: http://slack.com/12345
    channel: string
    username: string
- name: other_receiver
  slack_configs:
  - api_url: http://slack.com/54321
    channel: string
    username: string
- name: test_webhook
  webhook_configs:
  - url: http://webhook.com/12345
    send_resolved: true
- name: test_email
  email_configs:
  - to: test@mail.com
    from: testUser
    smarthost: http://mail-server.com
    headers:
      name: value
      foo: bar
templates:
- "path/to/file1"
- "path/to/file2"
- "path/to/file3"
`
)

func TestClient_CreateReceiver(t *testing.T) {
	client, fsClient, _ := newTestClient()
	// Create Slack Receiver
	err := client.CreateReceiver(testNID, tc.SampleSlackReceiver)
	assert.NoError(t, err)
	fsClient.AssertCalled(t, "WriteFile", "test/alertmanager.yml", mock.Anything, mock.Anything)

	// Create Webhook Receiver
	err = client.CreateReceiver(testNID, tc.SampleWebhookReceiver)
	assert.NoError(t, err)
	fsClient.AssertCalled(t, "WriteFile", "test/alertmanager.yml", mock.Anything, mock.Anything)

	// Create Email receiver
	err = client.CreateReceiver(testNID, tc.SampleEmailReceiver)
	assert.NoError(t, err)
	fsClient.AssertCalled(t, "WriteFile", "test/alertmanager.yml", mock.Anything, mock.Anything)

	// create duplicate receiver
	err = client.CreateReceiver(testNID, config.Receiver{Name: "receiver"})
	assert.Regexp(t, regexp.MustCompile("notification config name \".*receiver\" is not unique"), err.Error())
}

func TestClient_GetReceivers(t *testing.T) {
	client, _, _ := newTestClient()
	recs, err := client.GetReceivers(testNID)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(recs))
	assert.Equal(t, "receiver", recs[0].Name)
	assert.Equal(t, "slack", recs[1].Name)
	assert.Equal(t, "webhook", recs[2].Name)
	assert.Equal(t, "email", recs[3].Name)

	recs, err = client.GetReceivers(otherNID)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(recs))

	recs, err = client.GetReceivers("bad_nid")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(recs))
}

func TestClient_UpdateReceiver(t *testing.T) {
	client, fsClient, _ := newTestClient()
	err := client.UpdateReceiver(testNID, "slack", &config.Receiver{Name: "slack"})
	fsClient.AssertCalled(t, "WriteFile", "test/alertmanager.yml", mock.Anything, mock.Anything)
	assert.NoError(t, err)

	err = client.UpdateReceiver(testNID, "nonexistent", &config.Receiver{Name: "nonexistent"})
	fsClient.AssertNumberOfCalls(t, "WriteFile", 1)
	assert.Error(t, err)
}

func TestClient_DeleteReceiver(t *testing.T) {
	client, fsClient, _ := newTestClient()
	err := client.DeleteReceiver(testNID, "slack")
	fsClient.AssertCalled(t, "WriteFile", "test/alertmanager.yml", mock.Anything, mock.Anything)
	assert.NoError(t, err)

	err = client.DeleteReceiver(testNID, "nonexistent")
	assert.Error(t, err)
	fsClient.AssertNumberOfCalls(t, "WriteFile", 1)
}

func TestClient_ModifyTenantRoute(t *testing.T) {
	client, fsClient, _ := newTestClient()
	err := client.ModifyTenantRoute(testNID, &config.Route{
		Receiver: "test_tenant_base_route",
		Routes: []*config.Route{
			{Receiver: "slack"},
		},
	})
	assert.NoError(t, err)
	fsClient.AssertCalled(t, "WriteFile", "test/alertmanager.yml", mock.Anything, mock.Anything)

	err = client.ModifyTenantRoute(testNID, &config.Route{
		Receiver: "invalid_base_route",
		Routes: []*config.Route{
			{Receiver: "slack"},
		},
	})
	assert.EqualError(t, err, "route base receiver is incorrect (should be \"test_tenant_base_route\"). The base node should match nothing, then add routes as children of the base node")

	err = client.ModifyTenantRoute(testNID, &config.Route{
		Receiver: "test",
		Routes: []*config.Route{{
			Receiver: "nonexistent",
		}},
	})
	assert.Error(t, err)
	fsClient.AssertNumberOfCalls(t, "WriteFile", 1)
}

func TestClient_GetRoute(t *testing.T) {
	client, _, _ := newTestClient()

	route, err := client.GetRoute(otherNID)
	assert.NoError(t, err)
	assert.Equal(t, config.Route{Receiver: "other_tenant_base_route", Match: map[string]string{"tenantID": "other"}}, *route)

	_, err = client.GetRoute("no-network")
	assert.Error(t, err)
}

func TestClient_GetTenants(t *testing.T) {
	client, _, _ := newTestClient()

	tenants, err := client.GetTenants()
	assert.NoError(t, err)
	assert.Equal(t, []string{"other", "sample"}, tenants)
}

func TestClient_GetTemplateFileList(t *testing.T) {
	client, _, _ := newTestClient()

	tmpls, err := client.GetTemplateFileList()
	assert.NoError(t, err)
	assert.Equal(t, []string{"path/to/file1", "path/to/file2", "path/to/file3"}, tmpls)
}

func TestClient_AddTemplateFile(t *testing.T) {
	client, _, out := newTestClient()
	newPathName := "path/to/newFile"

	err := client.AddTemplateFile(newPathName)
	assert.NoError(t, err)

	newConf, _ := byteToConfig(*out)
	assert.Equal(t, len(newConf.Templates), 4)
	assert.Equal(t, newPathName, newConf.Templates[3])
}

func TestClient_RemoveTemplateFile(t *testing.T) {
	client, fsClient, out := newTestClient()

	// Remove existing path
	err := client.RemoveTemplateFile("path/to/file1")
	assert.NoError(t, err)

	newConf, _ := byteToConfig(*out)
	assert.Equal(t, len(newConf.Templates), 2)
	fsClient.AssertNumberOfCalls(t, "WriteFile", 1)

	// Remove non-existent path
	err = client.RemoveTemplateFile("path/to/noFile")
	assert.EqualError(t, err, "path not found: path/to/noFile")
	fsClient.AssertNumberOfCalls(t, "WriteFile", 1)
}

func newTestClient() (AlertmanagerClient, *mocks.FSClient, *[]byte) {
	fsClient := &mocks.FSClient{}
	fsClient.On("ReadFile", mock.Anything).Return([]byte(testAlertmanagerFile), nil)

	var outputFile []byte
	fsClient.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) { outputFile = args[1].([]byte) })
	tenancy := &alert.TenancyConfig{
		RestrictorLabel: "tenantID",
	}
	return NewClient("test/alertmanager.yml", "alertmanager-host:9093", tenancy, fsClient), fsClient, &outputFile
}

func byteToConfig(in []byte) (config.Config, error) {
	conf := config.Config{}
	return conf, yaml.Unmarshal(in, &conf)
}
