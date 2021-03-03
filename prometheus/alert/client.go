/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package alert

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"

	"github.com/facebookincubator/prometheus-configmanager/fsclient"

	"github.com/prometheus/prometheus/pkg/rulefmt"
)

const (
	rulesFilePostfix = "_rules.yml"
)

// PrometheusAlertClient provides thread-safe methods for writing, reading,
// and modifying alert configuration files
type PrometheusAlertClient interface {
	RuleExists(filePrefix, rulename string) bool
	WriteRule(filePrefix string, rule rulefmt.Rule) error
	UpdateRule(filePrefix string, rule rulefmt.Rule) error
	ReadRules(filePrefix, ruleName string) ([]rulefmt.Rule, error)
	DeleteRule(filePrefix, ruleName string) error
	BulkUpdateRules(filePrefix string, rules []rulefmt.Rule) (BulkUpdateResults, error)
	ReloadPrometheus() error
	Tenancy() TenancyConfig
}

type TenancyConfig struct {
	RestrictorLabel string `json:"restrictor_label"`
	RestrictQueries bool   `json:"restrict_queries"`
}

type client struct {
	fileLocks     *FileLocker
	prometheusURL string
	fsClient      fsclient.FSClient
	tenancy       TenancyConfig
}

func NewClient(fileLocks *FileLocker, prometheusURL string, fsClient fsclient.FSClient, tenancy TenancyConfig) PrometheusAlertClient {
	return &client{
		fileLocks:     fileLocks,
		prometheusURL: prometheusURL,
		fsClient:      fsClient,
		tenancy:       tenancy,
	}
}

// ValidateRule checks that a new alert rule is a valid specification
func ValidateRule(rule rulefmt.Rule) error {
	// convert to RuleNode for validation
	node := rulefmt.RuleNode{
		Record:      yaml.Node{Value: rule.Record},
		Alert:       yaml.Node{Value: rule.Alert},
		Expr:        yaml.Node{Value: rule.Expr},
		For:         0,
		Labels:      rule.Labels,
		Annotations: rule.Annotations,
	}
	if len(node.Validate()) != 0 {
		err := validateRuleImpl(node)
		glog.Errorf("Invalid rule: %v", err)
		return err
	}
	return nil
}

// validateRuleImpl determines the actual causes of the rule validation error.
// Due to how the underlying prometheus types are made (unexported), we have to copy this code
// and run it here to make it work. The actual validation is done with the package
// code.
func validateRuleImpl(r rulefmt.RuleNode) error {
	err := errors.New("Rule Validation Error")
	if r.Record.Value != "" && r.Alert.Value != "" {
		err = fmt.Errorf("%v; only one of 'record' and 'alert' must be set", err)
	}
	if r.Record.Value == "" && r.Alert.Value == "" {
		if r.Record.Value == "0" {
			err = fmt.Errorf("%v; one of 'record' or 'alert' must be set", err)
		} else {
			err = fmt.Errorf("%v; one of 'record' or 'alert' must be set", err)
		}
	}

	if r.Expr.Value == "" {
		err = fmt.Errorf("%v; field 'expr' must be set in rule", err)
	} else if _, e := parser.ParseExpr(r.Expr.Value); e != nil {
		err = fmt.Errorf("%v; could not parse expression: %v", err, e)
	}
	if r.Record.Value != "" {
		if len(r.Annotations) > 0 {
			err = fmt.Errorf("%v; invalid field 'annotations' in recording rule", err)
		}
		if r.For != 0 {
			err = fmt.Errorf("%v; invalid field 'for' in recording rule", err)
		}
		if !model.IsValidMetricName(model.LabelValue(r.Record.Value)) {
			err = fmt.Errorf("%v; invalid recording rule name: %s", err, r.Record.Value)
		}
	}

	for k, v := range r.Labels {
		if !model.LabelName(k).IsValid() || k == model.MetricNameLabel {
			err = fmt.Errorf("%v; invalid label name: %s", err, k)
		}

		if !model.LabelValue(v).IsValid() {
			err = fmt.Errorf("%v; invalid label value: %s", err, v)
		}
	}

	for k := range r.Annotations {
		if !model.LabelName(k).IsValid() {
			err = fmt.Errorf("%v; invalid annotation name: %s", err, k)
		}
	}
	return err
}

func (c *client) RuleExists(filePrefix, rulename string) bool {
	filename := makeFilename(filePrefix)

	c.fileLocks.Lock(filename)
	defer c.fileLocks.Unlock(filename)

	if !c.ruleFileExists(filename) {
		return false
	}
	ruleFile, err := c.readRuleFile(filename)
	if err != nil {
		return false
	}
	return ruleFile.GetRule(rulename) != nil
}

// WriteRule takes an alerting rule and writes it to the rules file for the
// given filePrefix
func (c *client) WriteRule(filePrefix string, rule rulefmt.Rule) error {
	filename := makeFilename(filePrefix)

	c.fileLocks.Lock(filename)
	defer c.fileLocks.Unlock(filename)

	ruleFile, err := c.readOrInitializeRuleFile(filePrefix, filename)
	if err != nil {
		return err
	}
	err = SecureRule(c.tenancy.RestrictQueries, c.tenancy.RestrictorLabel, filePrefix, &rule)
	if err != nil {
		return err
	}
	ruleFile.AddRule(rule)

	err = c.writeRuleFile(ruleFile, filename)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) UpdateRule(filePrefix string, rule rulefmt.Rule) error {
	filename := makeFilename(filePrefix)

	c.fileLocks.Lock(filename)
	defer c.fileLocks.Unlock(filename)

	ruleFile, err := c.readRuleFile(filename)
	if err != nil {
		return fmt.Errorf("rule file %s does not exist: %v", filename, err)
	}

	err = SecureRule(c.tenancy.RestrictQueries, c.tenancy.RestrictorLabel, filePrefix, &rule)
	if err != nil {
		return fmt.Errorf("cannot parse expression: \"%s\", %v", rule.Expr, err)
	}

	err = ruleFile.ReplaceRule(rule)
	if err != nil {
		return err
	}

	err = c.writeRuleFile(ruleFile, filename)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) ReadRules(filePrefix, ruleName string) ([]rulefmt.Rule, error) {
	filename := makeFilename(filePrefix)
	c.fileLocks.RLock(filename)
	defer c.fileLocks.RUnlock(filename)

	if !c.ruleFileExists(filename) {
		return []rulefmt.Rule{}, nil
	}

	ruleFile, err := c.readRuleFile(makeFilename(filePrefix))
	if err != nil {
		return []rulefmt.Rule{}, err
	}
	if ruleName == "" {
		return ruleFile.Rules(), nil
	}
	foundRule := ruleFile.GetRule(ruleName)
	if foundRule == nil {
		return nil, fmt.Errorf("rule %s not found", ruleName)
	}
	return []rulefmt.Rule{*foundRule}, nil
}

func (c *client) DeleteRule(filePrefix, ruleName string) error {
	filename := makeFilename(filePrefix)
	c.fileLocks.Lock(filename)
	defer c.fileLocks.Unlock(filename)

	ruleFile, err := c.readRuleFile(filename)
	if err != nil {
		return err
	}

	err = ruleFile.DeleteRule(ruleName)
	if err != nil {
		return err
	}

	err = c.writeRuleFile(ruleFile, filename)
	if err != nil {
		return err
	}
	return nil
}

func (c *client) BulkUpdateRules(filePrefix string, rules []rulefmt.Rule) (BulkUpdateResults, error) {
	filename := makeFilename(filePrefix)
	c.fileLocks.Lock(filename)
	defer c.fileLocks.Unlock(filename)

	ruleFile, err := c.readOrInitializeRuleFile(filePrefix, filename)
	if err != nil {
		return BulkUpdateResults{}, err
	}

	results := NewBulkUpdateResults()
	for _, newRule := range rules {
		ruleName := newRule.Alert

		err := SecureRule(c.tenancy.RestrictQueries, c.tenancy.RestrictorLabel, filePrefix, &newRule)
		if err != nil {
			results.Errors[ruleName] = err
			continue
		}

		if ruleFile.GetRule(ruleName) != nil {
			err := ruleFile.ReplaceRule(newRule)
			if err != nil {
				results.Errors[ruleName] = err
			} else {
				results.Statuses[ruleName] = "updated"
			}
		} else {
			ruleFile.AddRule(newRule)
			results.Statuses[ruleName] = "created"
		}
	}

	err = c.writeRuleFile(ruleFile, filename)
	if err != nil {
		return results, err
	}
	return results, nil
}

func (c *client) Tenancy() TenancyConfig {
	return c.tenancy
}

func (c *client) ReloadPrometheus() error {
	resp, err := http.Post(fmt.Sprintf("http://%s%s", c.prometheusURL, "/-/reload"), "text/plain", &bytes.Buffer{})
	if err != nil {
		glog.Errorf("error reloading prometheus: %v", err)
		return fmt.Errorf("error reloading prometheus: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		glog.Errorf("error reloading prometheus (status %d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("error reloading prometheus (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *client) writeRuleFile(ruleFile *File, filename string) error {
	yamlFile, err := yaml.Marshal(ruleFile)
	if err != nil {
		glog.Errorf("error writing rules file: %v", err)
		return fmt.Errorf("error writing rules file: %v", err)
	}
	err = c.fsClient.WriteFile(filename, yamlFile, 0666)
	if err != nil {
		glog.Errorf("error writing rules file: %v", err)
		return fmt.Errorf("error writing rules file: %v", err)
	}
	return nil
}

func (c *client) readOrInitializeRuleFile(filePrefix, filename string) (*File, error) {
	if c.ruleFileExists(filename) {
		return c.readRuleFile(filename)
	}
	return c.initializeRuleFile(filePrefix, filename)
}

func (c *client) initializeRuleFile(filePrefix, filename string) (*File, error) {
	if _, err := c.fsClient.Stat(filename); err == nil {
		file, err := c.readRuleFile(filename)
		if err != nil {
			return nil, err
		}
		return file, nil
	}
	return NewFile(filePrefix), nil
}

func (c *client) ruleFileExists(filename string) bool {
	_, err := c.fsClient.Stat(filename)
	return err == nil
}

func (c *client) readRuleFile(requestedFile string) (*File, error) {
	ruleFile := File{}
	file, err := c.fsClient.ReadFile(requestedFile)
	if err != nil {
		glog.Errorf("error reading rules file: %v", err)
		return &File{}, fmt.Errorf("error reading rules file: %v", err)
	}
	err = yaml.Unmarshal(file, &ruleFile)
	return &ruleFile, err
}

type BulkUpdateResults struct {
	Errors   map[string]error
	Statuses map[string]string
}

func NewBulkUpdateResults() BulkUpdateResults {
	return BulkUpdateResults{
		Errors:   make(map[string]error),
		Statuses: make(map[string]string),
	}
}

func (r BulkUpdateResults) String() string {
	str := strings.Builder{}
	if len(r.Errors) > 0 {
		str.WriteString("Errors: \n")
		names := funk.Keys(r.Errors).([]string)
		sort.Strings(names)
		for _, name := range names {
			str.WriteString(fmt.Sprintf("\t%s: %s\n", name, r.Errors[name]))
		}
	}
	if len(r.Statuses) > 0 {
		str.WriteString("Statuses: \n")
		names := funk.Keys(r.Statuses).([]string)
		sort.Strings(names)
		for _, name := range names {
			str.WriteString(fmt.Sprintf("\t%s: %s\n", name, r.Statuses[name]))
		}
	}
	return str.String()
}

func makeFilename(filePrefix string) string {
	return filePrefix + rulesFilePostfix
}
