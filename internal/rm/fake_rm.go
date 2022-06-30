/*
 * Copyright (c) 2019-2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY Type, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rm

import (
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/google/uuid"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var _ ResourceManager = (*resourceManager)(nil)

// resourceManager implements the ResourceManager interface
type fakeResourceManager struct {
	config   *spec.Config
	resource spec.ResourceName
	devices  Devices
}

// NewResourceManagers returns a []ResourceManager, one for each resource in 'config'.
func NewFakeResourceManagers(config *spec.Config) ResourceManager {
	return &fakeResourceManager{
		config:   config,
		resource: "nvidia.com/gpu",
		devices: Devices{
			"fakegpu0": &Device{
				Device: pluginapi.Device{
					ID:     uuid.New().String(),
					Health: "Healthy",
					Topology: &pluginapi.TopologyInfo{
						Nodes: []*pluginapi.NUMANode{},
					},
				},
				Paths: []string{},
			},
		},
	}
}

// IsFake
func (r *fakeResourceManager) IsFake() bool {
	return true
}

// Resource gets the resource name associated with the ResourceManager
func (r *fakeResourceManager) Resource() spec.ResourceName {
	return r.resource
}

// Resource gets the devices managed by the ResourceManager
func (r *fakeResourceManager) Devices() Devices {
	return r.devices
}

// CheckHealth performs health checks on a set of devices, writing to the 'unhealthy' channel with any unhealthy devices
func (r *fakeResourceManager) CheckHealth(stop <-chan interface{}, unhealthy chan<- *Device) error {
	for {
		select {
		case <-stop:
			return nil
		default:
		}
	}
}

// GetPreferredAllocation runs an allocation algorithm over the inputs.
// The algorithm chosen is based both on the incoming set of available devices and various config settings.
func (r *fakeResourceManager) GetPreferredAllocation(available, required []string, size int) ([]string, error) {
	return required, nil
}
