/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package config

import (
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
		Receivers: []*Receiver{{
			Name: "testReceiver",
		}},
	}
)

func TestConfig_RemoveReceiverFromRoute(t *testing.T) {
	copy := testConfig
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
