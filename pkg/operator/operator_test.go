// Copyright 2021 The Cluster Monitoring Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/cluster-monitoring-operator/pkg/client"
	"github.com/openshift/cluster-monitoring-operator/pkg/manifests"
	"github.com/pkg/errors"
)

func TestNewInfrastructureConfig(t *testing.T) {
	for _, tc := range []struct {
		name               string
		infrastructure     configv1.Infrastructure
		hostedControlPlane bool
		haInfrastructure   bool
	}{
		{
			name:               "empty infrastructure",
			infrastructure:     configv1.Infrastructure{},
			hostedControlPlane: false,
			haInfrastructure:   true,
		},
		{
			name: "External control plane",
			infrastructure: configv1.Infrastructure{
				Status: configv1.InfrastructureStatus{
					ControlPlaneTopology: configv1.ExternalTopologyMode,
				},
			},
			hostedControlPlane: true,
			haInfrastructure:   true,
		},
		{
			name: "Single-node infrastructure",
			infrastructure: configv1.Infrastructure{
				Status: configv1.InfrastructureStatus{
					InfrastructureTopology: configv1.SingleReplicaTopologyMode,
				},
			},
			hostedControlPlane: false,
			haInfrastructure:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewInfrastructureConfig(&tc.infrastructure)

			if c.HostedControlPlane() != tc.hostedControlPlane {
				t.Errorf("expected hosted control plane: %v, got %v", tc.hostedControlPlane, c.HostedControlPlane())
			}

			if c.HighlyAvailableInfrastructure() != tc.haInfrastructure {
				t.Errorf("expected HA infrastructure: %v, got %v", tc.haInfrastructure, c.HighlyAvailableInfrastructure())
			}
		})
	}
}

type proxyConfigCheckFunc func(*ProxyConfig) error

func proxyConfigChecks(fs ...proxyConfigCheckFunc) proxyConfigCheckFunc {
	return proxyConfigCheckFunc(func(c *ProxyConfig) error {
		for _, f := range fs {
			if err := f(c); err != nil {
				return err
			}
		}
		return nil
	})
}

func TestNewProxyConfig(t *testing.T) {
	hasHTTPProxy := func(expected string) proxyConfigCheckFunc {
		return proxyConfigCheckFunc(func(c *ProxyConfig) error {
			if got := c.HTTPProxy(); got != expected {
				return errors.Errorf("want http proxy %v, got %v", expected, got)
			}
			return nil
		})
	}

	hasHTTPSProxy := func(expected string) proxyConfigCheckFunc {
		return proxyConfigCheckFunc(func(c *ProxyConfig) error {
			if got := c.HTTPSProxy(); got != expected {
				return errors.Errorf("want https proxy %v, got %v", expected, got)
			}
			return nil
		})
	}

	hasNoProxy := func(expected string) proxyConfigCheckFunc {
		return proxyConfigCheckFunc(func(c *ProxyConfig) error {
			if got := c.NoProxy(); got != expected {
				return errors.Errorf("want noproxy %v, got %v", expected, got)
			}
			return nil
		})
	}

	for _, tc := range []struct {
		name  string
		p     *configv1.Proxy
		check proxyConfigCheckFunc
	}{
		{
			name: "empty spec",
			p:    &configv1.Proxy{},
			check: proxyConfigChecks(
				hasHTTPProxy(""),
				hasHTTPSProxy(""),
				hasNoProxy(""),
			),
		},
		{
			name: "proxies",
			p: &configv1.Proxy{
				Status: configv1.ProxyStatus{
					HTTPProxy:  "http://proxy",
					HTTPSProxy: "https://proxy",
					NoProxy:    "localhost,svc.cluster",
				},
			},
			check: proxyConfigChecks(
				hasHTTPProxy("http://proxy"),
				hasHTTPSProxy("https://proxy"),
				hasNoProxy("localhost,svc.cluster"),
			),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewProxyConfig(tc.p)

			if err := tc.check(c); err != nil {
				t.Error(err)
			}
		})
	}
}

func proxyReaderEquals(p1, p2 manifests.ProxyReader) bool {
	return p1.HTTPProxy() == p2.HTTPProxy() && p1.HTTPSProxy() == p2.HTTPSProxy() && p1.NoProxy() == p2.NoProxy()
}

func TestGetProxyReader(t *testing.T) {
	ctx := context.Background()
	emptyConfig := &manifests.Config{
		ClusterMonitoringConfiguration: &manifests.ClusterMonitoringConfiguration{
			HTTPConfig: &manifests.HTTPConfig{},
		},
	}
	nonEmptyConfig := &manifests.Config{
		ClusterMonitoringConfiguration: &manifests.ClusterMonitoringConfiguration{
			HTTPConfig: &manifests.HTTPConfig{
				HTTPProxy: "foo",
			},
		},
	}
	proxyConfig := &ProxyConfig{}
	for _, tc := range []struct {
		name                string
		proxyConfigSupplier proxyConfigSupplier
		config              *manifests.Config
		expectedProxyReader manifests.ProxyReader
	}{
		{
			name:                "A non empty CMO configmap proxy configuration should get priority over the cluster-wide proxy configuration",
			proxyConfigSupplier: func(ctx context.Context) (*ProxyConfig, error) { return nil, nil },
			config:              nonEmptyConfig,
			expectedProxyReader: nonEmptyConfig,
		},
		{
			name:                "An empty CMO configmap proxy configuration should not get priority over the cluster-wide proxy configuration",
			proxyConfigSupplier: func(ctx context.Context) (*ProxyConfig, error) { return proxyConfig, nil },
			config:              emptyConfig,
			expectedProxyReader: proxyConfig,
		},
		{
			name:                "An empty proxy configuration should be used as default if the CMO configmap proxy configuration is empty and we fail to read the cluster-wide proxy configuration",
			proxyConfigSupplier: func(ctx context.Context) (*ProxyConfig, error) { return proxyConfig, errors.New("forced error") },
			config:              emptyConfig,
			expectedProxyReader: emptyConfig,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proxyReader := getProxyReader(ctx, tc.config, tc.proxyConfigSupplier)
			if !proxyReaderEquals(proxyReader, tc.expectedProxyReader) {
				t.Error()
			}
		})
	}
}

func isNilOrAsExpected(s client.StateInfo) bool {
	if s == nil {
		return true
	}

	switch s.(type) {
	case *expectedStatus:
		return true
	default:
		return false
	}
}

func isDegraded(r runReport) bool {
	if isNilOrAsExpected(r.degraded) {
		return false
	}
	return true
}

func isUnavailable(r runReport) bool {
	if isNilOrAsExpected(r.available) {
		return false
	}
	return true
}

func TestRunReport(t *testing.T) {
	tt := []struct {
		name       string
		degraded   client.StateInfo
		isDegraded bool

		available     client.StateInfo
		isUnavailable bool
	}{{
		name:     "all nils",
		degraded: nil, isDegraded: false,
		available: nil, isUnavailable: false,
	}, {
		name:     "degraded: false",
		degraded: asExpected(client.FalseStatus), isDegraded: false,
		available: nil, isUnavailable: false,
	}, {
		name:     "available: false",
		degraded: nil, isDegraded: false,
		available: asExpected(client.TrueStatus), isUnavailable: false,
	}, {
		name:     "degraded: stateInfo",
		degraded: &stateInfo{}, isDegraded: true,
		available: nil, isUnavailable: false,
	}, {
		name:     "available: stateInfo",
		degraded: nil, isDegraded: false,
		available: &stateInfo{}, isUnavailable: true,
	}}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			rr := runReport{
				degraded:  tc.degraded,
				available: tc.available,
			}

			if got, want := isDegraded(rr), tc.isDegraded; got != want {
				t.Errorf("expected degraded to be %t but got %t", want, got)
			}

			if got, want := isUnavailable(rr), tc.isUnavailable; got != want {
				t.Errorf("expected degraded to be %t but got %t", want, got)
			}
		})

	}
}
