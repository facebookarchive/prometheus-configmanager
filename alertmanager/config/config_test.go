/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package config

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testConfig = Config{
		Global: nil,
		Route: &Route{
			Receiver: "base",
			Routes: []*Route{
				{Receiver: "testReceiver"},
				{Receiver: "testReceiver2"},
				{
					Receiver: "testReceiver3",
					Routes: []*Route{
						{Receiver: "testReceiver"},
						{Receiver: "testReceiverChild1"},
					},
				},
			},
		},
		Receivers: []*Receiver{
			{Name: "base"},
			{Name: "testReceiver"},
			{Name: "testReceiver2"},
			{Name: "testReceiver3"},
			{Name: "testReceiverChild1"},
		},
	}
)

func TestConfig_RemoveReceiverFromRoute(t *testing.T) {
	copy := deepCopy(testConfig)
	copy.RemoveReceiverFromRoute("testReceiver")
	assert.Len(t, copy.Route.Routes, 2)
	assert.Equal(t, copy.Route.Routes[0].Receiver, "testReceiver2")
	assert.Equal(t, copy.Route.Routes[1].Receiver, "testReceiver3")

	assert.Len(t, copy.Route.Routes[1].Routes, 1)
	assert.Equal(t, copy.Route.Routes[1].Routes[0].Receiver, "testReceiverChild1")
}

func TestConfig_SearchRoutesForReceiver(t *testing.T) {
	assert.True(t, testConfig.SearchRoutesForReceiver("base"))
	assert.True(t, testConfig.SearchRoutesForReceiver("testReceiver2"))
	assert.True(t, testConfig.SearchRoutesForReceiver("testReceiver3"))
	assert.True(t, testConfig.SearchRoutesForReceiver("testReceiverChild1"))
	assert.False(t, testConfig.SearchRoutesForReceiver("foo"))
}

func TestConfig_InitializeBaseRoute(t *testing.T) {
	newRoute := &Route{
		Receiver: "test",
		Match:    map[string]string{"tenant": "test"},
	}
	copy := deepCopy(testConfig)
	err := copy.InitializeNetworkBaseRoute(newRoute, "testMatcher", "tenant1")
	assert.True(t, copy.SearchRoutesForReceiver("tenant1_tenant_base_route"))
	assert.Equal(t, copy.Route.Routes[3].Receiver, "tenant1_tenant_base_route")
	assert.Equal(t, copy.Route.Routes[3].Match["testMatcher"], "tenant1")
	assert.NoError(t, err)

	err = copy.InitializeNetworkBaseRoute(newRoute, "testMatcher", "tenant1")
	assert.EqualError(t, err, "Base route for tenant tenant1 already exists")
}

func deepCopy(conf Config) (new Config) {
	b, _ := json.Marshal(conf)
	err := json.Unmarshal(b, &new)
	if err != nil {
		panic(fmt.Errorf("this shouldn't happen: %v", err))
	}
	return new
}
