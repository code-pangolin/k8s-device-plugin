package main

import (
	spec "github.com/NVIDIA/k8s-device-plugin/api/config/v1"
	"github.com/NVIDIA/k8s-device-plugin/internal/rm"
	"github.com/google/uuid"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// NewNvidiaDevicePlugin returns an initialized NvidiaDevicePlugin
func NewFakeNvidiaDevicePlugin(config *spec.Config, resourceManager rm.ResourceManager) *NvidiaDevicePlugin {
	_, name := resourceManager.Resource().Split()

	return &NvidiaDevicePlugin{
		rm:               resourceManager,
		config:           config,
		deviceListEnvvar: "NVIDIA_VISIBLE_DEVICES",
		socket:           pluginapi.DevicePluginPath + "nvidia-" + name + ".sock",

		// These will be reinitialized every
		// time the plugin server is restarted.
		server: nil,
		health: nil,
		stop:   nil,
		uuid:   uuid.New().String(),
	}
}
