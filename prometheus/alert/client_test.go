/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package alert_test

import (
	"errors"
	"testing"

	"github.com/facebookincubator/prometheus-configmanager/fsclient/mocks"
	"github.com/facebookincubator/prometheus-configmanager/prometheus/alert"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testNID      = "test"
	testRuleFile = `groups:
- name: test
  rules:
  - alert: test_rule_1
    expr: up == 0{tenantID="test"}
    for: 5s
    labels:
      severity: major
      tenantID: test
  - alert: test_rule_2
    expr: up == 1{tenantID="test"}
    for: 5s
    labels:
      severity: critical
      tenantID: test
    annotations:
      summary: A test rule`

	otherNID      = "other"
	otherRuleFile = `groups:
- name: other
  rules:
  - alert: other_rule_1
    expr: up == 0{tenantID="other"}
    for: 5s
    labels:
      severity: major
      tenantID: other
  - alert: test_rule_2
    expr: up == 1{tenantID="other"}
    for: 5s
    labels:
      severity: critical
      tenantID: other
    annotations:
      summary: A test rule`
)

var (
	fiveSeconds, _ = model.ParseDuration("5s")
	testRule1      = rulefmt.Rule{
		Alert:  "test_rule_1",
		Expr:   "up==0",
		For:    fiveSeconds,
		Labels: map[string]string{"severity": "major", "tenantID": testNID},
	}
	badRule = rulefmt.Rule{
		Alert: "bad_rule",
		Expr:  "malformed{.}",
	}

	// Mock FSClient that never errs
	healthyFSClient  = newFSClient(nil, nil)
	readErrFSClient  = newFSClient(errors.New("read err"), nil)
	writeErrFSClient = newFSClient(nil, errors.New("write err"))
)

type validateRuleTestCase struct {
	name          string
	rule          rulefmt.Rule
	expectedError string
}

func (tc *validateRuleTestCase) RunTest(t *testing.T) {
	err := alert.ValidateRule(tc.rule)
	if tc.expectedError == "" {
		assert.NoError(t, err)
		return
	}
	assert.EqualError(t, err, tc.expectedError)
}

func TestValidateRule(t *testing.T) {
	tests := []validateRuleTestCase{
		{
			name: "valid rule",
			rule: rulefmt.Rule{
				Alert:       "test",
				Expr:        "up",
				For:         0,
				Labels:      map[string]string{"label1": "value"},
				Annotations: map[string]string{"annotation1": "value"},
			},
		},
		{
			name:          "record and alert defined",
			rule:          rulefmt.Rule{Alert: "alert", Record: "record"},
			expectedError: "Rule Validation Error; only one of 'record' and 'alert' must be set; field 'expr' must be set in rule",
		},
		{
			name:          "neither defined",
			rule:          rulefmt.Rule{Alert: "", Record: ""},
			expectedError: "Rule Validation Error; one of 'record' or 'alert' must be set; field 'expr' must be set in rule",
		},
		{
			name:          "no expression",
			rule:          rulefmt.Rule{Alert: "test", Expr: ""},
			expectedError: "Rule Validation Error; field 'expr' must be set in rule",
		},
		{
			name:          "invalid expression",
			rule:          rulefmt.Rule{Alert: "test", Expr: "!up"},
			expectedError: "Rule Validation Error; could not parse expression: 1:1: parse error: unexpected character after '!': 'u'",
		},
		{
			name:          "annotions in recording rule",
			rule:          rulefmt.Rule{Record: "test", Expr: "up", Annotations: map[string]string{"a": "b"}},
			expectedError: "Rule Validation Error; invalid field 'annotations' in recording rule",
		},
		{
			name:          "invalid recording rule name",
			rule:          rulefmt.Rule{Record: "1test", Expr: "up"},
			expectedError: "Rule Validation Error; invalid recording rule name: 1test",
		},
		{
			name:          "invalid label name",
			rule:          rulefmt.Rule{Alert: "test", Expr: "up", Labels: map[string]string{"1label": "val"}},
			expectedError: "Rule Validation Error; invalid label name: 1label",
		},
		{
			name:          "invalid annotation name",
			rule:          rulefmt.Rule{Alert: "test", Expr: "up", Annotations: map[string]string{"1label": "val"}},
			expectedError: "Rule Validation Error; invalid annotation name: 1label",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.RunTest)
	}
}

func TestClient_RuleExists(t *testing.T) {
	client := newTestClient("tenantID", healthyFSClient)
	assert.True(t, client.RuleExists(testNID, "test_rule_1"))
	assert.True(t, client.RuleExists(testNID, "test_rule_2"))
	assert.False(t, client.RuleExists(testNID, "no_rule"))
	assert.False(t, client.RuleExists(testNID, "other_rule_1"))

	assert.True(t, client.RuleExists(otherNID, "other_rule_1"))
	assert.True(t, client.RuleExists(otherNID, "test_rule_2"))
	assert.False(t, client.RuleExists(otherNID, "no_rule"))
	assert.False(t, client.RuleExists(otherNID, "test_rule_1"))

	assert.False(t, client.RuleExists("not_a_file", "no_rule"))

	// Stat error
	assert.False(t, client.RuleExists("not_a_file", "no_rule"))
}

func TestClient_WriteRule(t *testing.T) {
	client := newTestClient("tenantID", healthyFSClient)
	err := client.WriteRule(testNID, sampleRule)
	assert.NoError(t, err)
	// cannot secure rule
	err = client.WriteRule(testNID, badRule)
	assert.EqualError(t, err, "error parsing query: 1:11: parse error: unexpected character inside braces: '.'")
	// initialize new file
	err = client.WriteRule("newPrefix", sampleRule)
	assert.NoError(t, err)
	// file does not exist
	client = newTestClient("tenantID", readErrFSClient)
	err = client.WriteRule(testNID, testRule1)
	assert.EqualError(t, err, "error reading rules file: read err")
	// error writing file
	client = newTestClient("tenantID", writeErrFSClient)
	err = client.WriteRule(testNID, testRule1)
	assert.EqualError(t, err, "error writing rules file: write err")
}

func TestClient_UpdateRule(t *testing.T) {
	client := newTestClient("tenantID", healthyFSClient)
	err := client.UpdateRule(testNID, testRule1)
	assert.NoError(t, err)
	// Returns error when updating non-existent rule
	err = client.UpdateRule(testNID, sampleRule)
	assert.Error(t, err)
	// cannot secure rule
	err = client.UpdateRule(testNID, badRule)
	assert.EqualError(t, err, "cannot parse expression: \"malformed{.}\", error parsing query: 1:11: parse error: unexpected character inside braces: '.'")
	// file does not exist
	client = newTestClient("tenantID", readErrFSClient)
	err = client.UpdateRule(testNID, testRule1)
	assert.EqualError(t, err, "rule file test_rules.yml does not exist: error reading rules file: read err")
	// error writing file
	client = newTestClient("tenantID", writeErrFSClient)
	err = client.UpdateRule(testNID, testRule1)
	assert.EqualError(t, err, "error writing rules file: write err")
}

func TestClient_ReadRules(t *testing.T) {
	client := newTestClient("tenantID", healthyFSClient)

	rules, err := client.ReadRules(testNID, "")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rules))
	assert.Equal(t, "test_rule_1", rules[0].Alert)
	assert.Equal(t, "test_rule_2", rules[1].Alert)

	rules, err = client.ReadRules(otherNID, "")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rules))
	assert.Equal(t, "other_rule_1", rules[0].Alert)
	assert.Equal(t, "test_rule_2", rules[1].Alert)

	rules, err = client.ReadRules(testNID, "test_rule_1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(rules))
	assert.Equal(t, "test_rule_1", rules[0].Alert)

	rules, err = client.ReadRules(testNID, "no_rule")
	assert.Error(t, err)
	assert.Equal(t, 0, len(rules))

	// rule file doesn't exist
	rules, err = client.ReadRules("not_a_file", "")
	assert.NoError(t, err)
	assert.Equal(t, rules, []rulefmt.Rule{})
}

func TestClient_DeleteRule(t *testing.T) {
	client := newTestClient("tenantID", healthyFSClient)
	err := client.DeleteRule(testNID, "test_rule_1")
	assert.NoError(t, err)

	err = client.DeleteRule(testNID, "no_rule")
	assert.Error(t, err)

	// cannot read file
	client = newTestClient("tenantID", readErrFSClient)
	err = client.DeleteRule(testNID, "test_rule_1")
	assert.EqualError(t, err, "error reading rules file: read err")

	// cannot write file
	client = newTestClient("tenantID", writeErrFSClient)
	err = client.DeleteRule(testNID, "test_rule_1")
	assert.EqualError(t, err, "error writing rules file: write err")
}

func TestClient_BulkUpdateRules(t *testing.T) {
	client := newTestClient("tenantID", healthyFSClient)
	results, err := client.BulkUpdateRules(testNID, []rulefmt.Rule{sampleRule, testRule1})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(results.Statuses))
	assert.Equal(t, 0, len(results.Errors))

	results, err = client.BulkUpdateRules(testNID, []rulefmt.Rule{badRule, sampleRule, testRule1})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(results.Statuses))
	assert.Equal(t, 1, len(results.Errors))
	// Check results string
	assert.Equal(t, "Errors: \n\tbad_rule: error parsing query: 1:11: parse error: unexpected character inside braces: '.'\nStatuses: \n\ttestAlert: created\n\ttest_rule_1: updated\n", results.String())

	// cannot read file
	client = newTestClient("tenantID", readErrFSClient)
	results, err = client.BulkUpdateRules(testNID, []rulefmt.Rule{sampleRule})
	assert.EqualError(t, err, "error reading rules file: read err")

	// cannot write file
	client = newTestClient("tenantID", writeErrFSClient)
	results, err = client.BulkUpdateRules(testNID, []rulefmt.Rule{sampleRule})
	assert.EqualError(t, err, "error writing rules file: write err")
}

func newTestClient(multitenantLabel string, fsClient *mocks.FSClient) alert.PrometheusAlertClient {
	dClient := newHealthyDirClient("test")
	fileLocks, _ := alert.NewFileLocker(dClient)
	tenancy := alert.TenancyConfig{
		RestrictorLabel: multitenantLabel,
		RestrictQueries: true,
	}
	return alert.NewClient(fileLocks, "prometheus-host.com", fsClient, tenancy)
}

func newFSClient(readFileErr, writeFileErr error) *mocks.FSClient {
	fsClient := &mocks.FSClient{}
	fsClient.On("Stat", "test_rules.yml").Return(nil, nil)
	fsClient.On("Stat", "other_rules.yml").Return(nil, nil)
	fsClient.On("Stat", mock.AnythingOfType("string")).Return(nil, errors.New("file not found"))
	fsClient.On("ReadFile", "test_rules.yml").Return([]byte(testRuleFile), readFileErr)
	fsClient.On("ReadFile", "other_rules.yml").Return([]byte(otherRuleFile), readFileErr)
	fsClient.On("ReadFile", mock.AnythingOfType("string")).Return([]byte{}, errors.New("file does not exist"))
	fsClient.On("WriteFile", mock.Anything, mock.Anything, mock.Anything).Return(writeFileErr)
	fsClient.On("Root").Return("test_rules/")
	return fsClient
}
