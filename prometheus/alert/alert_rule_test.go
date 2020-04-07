/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package alert_test

import (
	"testing"

	"prometheus-configmanager/prometheus/alert"
	"prometheus-configmanager/restrictor"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"github.com/stretchr/testify/assert"
)

const (
	alertName  = "testAlert"
	alertName2 = "testAlert2"
)

var (
	sampleRule = rulefmt.Rule{
		Alert:  alertName,
		Expr:   "up == 0",
		Labels: map[string]string{"name": "value"},
	}

	sampleRule2 = rulefmt.Rule{
		Alert:  alertName2,
		Expr:   "up == 0",
		Labels: map[string]string{"name": "value"},
	}
)

func TestFile_GetRule(t *testing.T) {
	f := sampleFile()

	rule := f.GetRule(alertName)
	assert.Equal(t, sampleRule, *rule)

	rule = f.GetRule("")
	assert.Equal(t, nil, nil)
}

func TestFile_AddRule(t *testing.T) {
	f := sampleFile()

	f.AddRule(sampleRule2)
	assert.Equal(t, 2, len(f.Rules()))
	assert.NotNil(t, f.GetRule(alertName))
	assert.NotNil(t, f.GetRule(alertName2))
}

func TestFile_ReplaceRule(t *testing.T) {
	f := sampleFile()
	newRule := rulefmt.Rule{
		Alert: alertName,
		Expr:  "up == 1",
	}
	err := f.ReplaceRule(newRule)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(f.Rules()))
	assert.Equal(t, newRule, *f.GetRule(alertName))

	badRule := rulefmt.Rule{
		Alert: "badRule",
	}

	err = f.ReplaceRule(badRule)
	assert.Error(t, err)
}

func TestFile_DeleteRule(t *testing.T) {
	f := sampleFile()
	err := f.DeleteRule(alertName)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(f.Rules()))

	// error if deleting non-existent rule
	err = f.DeleteRule(alertName)
	assert.Error(t, err)
}

func TestSecureRule(t *testing.T) {
	rule := sampleRule
	err := alert.SecureRule(true, "tenantID", "test", &rule)
	assert.NoError(t, err)

	queryRestrictor := restrictor.NewQueryRestrictor(restrictor.DefaultOpts).AddMatcher("tenantID", "test")
	expectedExpr, _ := queryRestrictor.RestrictQuery(sampleRule.Expr)

	assert.Equal(t, expectedExpr, rule.Expr)
	assert.Equal(t, 2, len(rule.Labels))
	assert.Equal(t, "test", rule.Labels["tenantID"])

	existingNetworkIDRule := rulefmt.Rule{
		Alert:  alertName2,
		Expr:   `up{tenantID="test"} == 0`,
		Labels: map[string]string{"name": "value", "tenantID": "test"},
	}
	restricted, _ := queryRestrictor.RestrictQuery(existingNetworkIDRule.Expr)
	// assert tenantID isn't appended twice
	assert.Equal(t, expectedExpr, restricted)
	assert.Equal(t, 2, len(rule.Labels))

	// assert expression is not restricted when restrictQueries is false
	rule = sampleRule
	origRule := sampleRule.Expr
	err = alert.SecureRule(false, "tenantID", "test", &rule)
	assert.NoError(t, err)
	assert.Equal(t, origRule, rule.Expr)

	rule = rulefmt.Rule{
		Alert: "test",
		Expr:  "up == 0",
	}
	restricted, _ = queryRestrictor.RestrictQuery(rule.Expr)
	err = alert.SecureRule(true, "tenantID", "test", &rule)
	assert.NoError(t, err)
	assert.Equal(t, restricted, rule.Expr)
	assert.Equal(t, 1, len(rule.Labels))
	assert.Equal(t, "test", rule.Labels["tenantID"])
}

func TestRuleJSONWrapper_ToRuleFmt(t *testing.T) {
	jsonRule := alert.RuleJSONWrapper{
		Record:      "record",
		Alert:       "alert",
		Expr:        "expr",
		For:         "5s",
		Labels:      nil,
		Annotations: nil,
	}

	expectedFor, _ := model.ParseDuration("5s")
	expectedRule := rulefmt.Rule{
		Record:      "record",
		Alert:       "alert",
		Expr:        "expr",
		For:         expectedFor,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}

	actualRule, err := jsonRule.ToRuleFmt()
	assert.NoError(t, err)
	assert.Equal(t, expectedRule, actualRule)
}

func sampleFile() alert.File {
	return alert.File{
		RuleGroups: []rulefmt.RuleGroup{{
			Name:  "testGroup",
			Rules: []rulefmt.Rule{sampleRule},
		},
		},
	}
}
