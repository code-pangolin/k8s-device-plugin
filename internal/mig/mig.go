// Copyright (c) 2021 - 2022, NVIDIA CORPORATION. All rights reserved.

package mig

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
)

const (
	nvidiaProcDriverPath   = "/proc/driver/nvidia"
	nvidiaCapabilitiesPath = nvidiaProcDriverPath + "/capabilities"

	nvcapsProcDriverPath = "/proc/driver/nvidia-caps"
	nvcapsMigMinorsPath  = nvcapsProcDriverPath + "/mig-minors"
	nvcapsDevicePath     = "/dev/nvidia-caps"
)

// DeviceInfo stores information about all devices on the node
type DeviceInfo struct {
	// devicesMap holds a list of devices, separated by whether they have MigEnabled or not
	devicesMap map[bool][]*nvml.Device
}

// NewDeviceInfo creates a new DeviceInfo struct and returns a pointer to it.
func NewDeviceInfo() *DeviceInfo {
	return &DeviceInfo{
		devicesMap: nil, // Is initialized on first use
	}
}

func (devices *DeviceInfo) getDevicesMap() (map[bool][]*nvml.Device, error) {
	if devices.devicesMap == nil {
		n, err := nvml.GetDeviceCount()
		if err != nil {
			return nil, err
		}

		migEnabledDevicesMap := make(map[bool][]*nvml.Device)
		for i := uint(0); i < n; i++ {
			d, err := nvml.NewDeviceLite(i)
			if err != nil {
				return nil, err
			}

			isMigEnabled, err := d.IsMigEnabled()
			if err != nil {
				return nil, err
			}

			migEnabledDevicesMap[isMigEnabled] = append(migEnabledDevicesMap[isMigEnabled], d)
		}

		devices.devicesMap = migEnabledDevicesMap
	}
	return devices.devicesMap, nil
}

// GetDevicesWithMigEnabled returns a list of devices with migEnabled=true
func (devices *DeviceInfo) GetDevicesWithMigEnabled() ([]*nvml.Device, error) {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return nil, err
	}
	return devicesMap[true], nil
}

// GetDevicesWithMigDisabled returns a list of devices with migEnabled=false
func (devices *DeviceInfo) GetDevicesWithMigDisabled() ([]*nvml.Device, error) {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return nil, err
	}
	return devicesMap[false], nil
}

// AssertAllMigEnabledDevicesAreValid ensures that all devices with migEnabled=true are valid.
// This means:
// * They have at least 1 mig devices associated with them
// * If (uniform == true) all MIG devices are associated with the same profile
// Returns nil if the device is valid, or an error if these are not valid
func (devices *DeviceInfo) AssertAllMigEnabledDevicesAreValid(uniform bool) error {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return err
	}

	var previousAttrs *nvml.DeviceAttributes
	for _, d := range devicesMap[true] {
		migs, err := d.GetMigDevices()
		if err != nil {
			return err
		}
		if len(migs) == 0 {
			return fmt.Errorf("No MIG devices associated with %v: %v", d.Path, d.UUID)
		}
		if !uniform {
			continue
		}
		for _, m := range migs {
			attrs, err := m.GetAttributes()
			if err != nil {
				return err
			}
			if previousAttrs == nil {
				previousAttrs = &attrs
			}
			if attrs != *previousAttrs {
				return fmt.Errorf("More than one MIG device type present on node")
			}
		}
	}

	return nil
}

// GetAllMigDevices returns a list of all MIG devices.
func (devices *DeviceInfo) GetAllMigDevices() ([]*nvml.Device, error) {
	devicesMap, err := devices.getDevicesMap()
	if err != nil {
		return nil, err
	}

	var migs []*nvml.Device
	for _, d := range devicesMap[true] {
		devs, err := d.GetMigDevices()
		if err != nil {
			return nil, err
		}
		migs = append(migs, devs...)
	}
	return migs, nil
}

// GetMigDevicePartsByUUID returns the parent GPU UUID and GI and CI ids of the MIG device.
func GetMigDevicePartsByUUID(uuid string) (string, uint, uint, error) {
	return nvml.ParseMigDeviceUUID(uuid)
}

// GetMigCapabilityDevicePaths returns a mapping of MIG capability path to device node path
func GetMigCapabilityDevicePaths() (map[string]string, error) {
	// Open nvcapsMigMinorsPath for walking.
	// If the nvcapsMigMinorsPath does not exist, then we are not on a MIG
	// capable machine, so there is nothing to do.
	// The format of this file is discussed in:
	//     https://docs.nvidia.com/datacenter/tesla/mig-user-guide/index.html#unique_1576522674
	minorsFile, err := os.Open(nvcapsMigMinorsPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error opening MIG minors file: %v", err)
	}
	defer minorsFile.Close()

	// Define a function to process each each line of nvcapsMigMinorsPath
	processLine := func(line string) (string, int, error) {
		var gpu, gi, ci, migMinor int

		// Look for a CI access file
		n, _ := fmt.Sscanf(line, "gpu%d/gi%d/ci%d/access %d", &gpu, &gi, &ci, &migMinor)
		if n == 4 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath+"/gpu%d/mig/gi%d/ci%d/access", gpu, gi, ci)
			return capPath, migMinor, nil
		}

		// Look for a GI access file
		n, _ = fmt.Sscanf(line, "gpu%d/gi%d/access %d", &gpu, &gi, &migMinor)
		if n == 3 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath+"/gpu%d/mig/gi%d/access", gpu, gi)
			return capPath, migMinor, nil
		}

		// Look for the MIG config file
		n, _ = fmt.Sscanf(line, "config %d", &migMinor)
		if n == 1 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath + "/mig/config")
			return capPath, migMinor, nil
		}

		// Look for the MIG monitor file
		n, _ = fmt.Sscanf(line, "monitor %d", &migMinor)
		if n == 1 {
			capPath := fmt.Sprintf(nvidiaCapabilitiesPath + "/mig/monitor")
			return capPath, migMinor, nil
		}

		return "", 0, fmt.Errorf("unparsable line: %v", line)
	}

	// Walk each line of nvcapsMigMinorsPath and construct a mapping of nvidia
	// capabilities path to device minor for that capability
	capsDevicePaths := make(map[string]string)
	scanner := bufio.NewScanner(minorsFile)
	for scanner.Scan() {
		capPath, migMinor, err := processLine(scanner.Text())
		if err != nil {
			log.Printf("Skipping line in MIG minors file: %v", err)
			continue
		}
		capsDevicePaths[capPath] = fmt.Sprintf(nvcapsDevicePath+"/nvidia-cap%d", migMinor)
	}
	return capsDevicePaths, nil
}

// GetMigDeviceNodePaths returns a list of device node paths associated with a MIG device
func GetMigDeviceNodePaths(uuid string) ([]string, error) {
	capDevicePaths, err := GetMigCapabilityDevicePaths()
	if err != nil {
		return nil, fmt.Errorf("error getting MIG capability device paths: %v", err)
	}

	parentUUID, gi, ci, err := nvml.ParseMigDeviceUUID(uuid)
	if err != nil {
		return nil, fmt.Errorf("error separating MIG device into its constituent parts: %v", err)
	}

	parent, err := nvml.NewDeviceLiteByUUID(parentUUID)
	if err != nil {
		return nil, fmt.Errorf("error getting parent for MIG device with UUID '%v': %v", uuid, err)
	}

	var gpu int
	_, err = fmt.Sscanf(parent.Path, "/dev/nvidia%d", &gpu)
	if err != nil {
		return nil, fmt.Errorf("error getting GPU minor: %v", err)
	}

	giCapPath := fmt.Sprintf(nvidiaCapabilitiesPath+"/gpu%d/mig/gi%d/access", gpu, gi)
	if _, exists := capDevicePaths[giCapPath]; !exists {
		return nil, fmt.Errorf("missing MIG GPU instance capability path: %v", giCapPath)
	}

	ciCapPath := fmt.Sprintf(nvidiaCapabilitiesPath+"/gpu%d/mig/gi%d/ci%d/access", gpu, gi, ci)
	if _, exists := capDevicePaths[ciCapPath]; !exists {
		return nil, fmt.Errorf("missing MIG GPU instance capability path: %v", giCapPath)
	}

	devicePaths := []string{
		parent.Path,
		capDevicePaths[giCapPath],
		capDevicePaths[ciCapPath],
	}

	return devicePaths, nil
}
