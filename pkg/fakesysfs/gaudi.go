/*
 * Copyright (c) 2024, Intel Corporation.  All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package fakesysfs

import (
	"fmt"
	"os"
	"path"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/device"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/helpers"
	"golang.org/x/sys/unix"
)

const (
	devNullMajor = 1
	devNullMinor = 3
	devNullType  = unix.S_IFCHR
)

func FakeSysFsGaudiContents(sysfsRoot string, devfsRoot string, gaudis device.DevicesInfo) error {
	if err := sanitizeFakeSysFsDir(sysfsRoot); err != nil {
		return err
	}

	return fakeSysFsGaudiDevices(sysfsRoot, devfsRoot, gaudis)
}

// fakeSysFsGaudiDevices creates PCI and DRM devices layout in existing fake sysfsRoot.
// This will be called when fake sysfs is being created and when more devices added
// to existing fake sysfs.
func fakeSysFsGaudiDevices(sysfsRoot string, devfsRoot string, gaudis device.DevicesInfo) error {
	for _, gaudi := range gaudis {
		// bus/pci/driver/<device> setup
		pciDriverDevDir := path.Join(sysfsRoot, "bus/pci/drivers/habanalabs/", gaudi.PCIAddress)
		if err := os.MkdirAll(pciDriverDevDir, 0755); err != nil {
			return fmt.Errorf("creating fake sysfs, err: %v", err)
		}

		if writeErr := helpers.WriteFile(path.Join(pciDriverDevDir, "device"), gaudi.Model); writeErr != nil {
			return fmt.Errorf("creating fake sysfs dir, err: %v", writeErr)
		}

		deviceName := fmt.Sprintf("accel%v", gaudi.DeviceIdx)
		controlDeviceName := fmt.Sprintf("accel_controlD%v", gaudi.DeviceIdx)
		// devices/virtual/accel/<device> setup
		dirPath := path.Join(sysfsRoot, "devices/virtual/accel", deviceName, "device")
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("creating fake sysfs dir, err: %v", err)
		}
		// $ cat /sys/devices/virtual/accel/accel0/device/pci_addr
		// 0000:0f:00.0
		if writeErr := helpers.WriteFile(path.Join(dirPath, "pci_addr"), gaudi.PCIAddress); writeErr != nil {
			return fmt.Errorf("creating fake sysfs dir, err: %v", writeErr)
		}

		if writeErr := helpers.WriteFile(path.Join(dirPath, "module_id"), fmt.Sprintf("%v", gaudi.DeviceIdx)); writeErr != nil {
			return fmt.Errorf("creating fake sysfs dir, err: %v", writeErr)
		}

		dirPath = path.Join(sysfsRoot, "devices/virtual/accel", controlDeviceName)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("creating fake sysfs, err: %v", err)
		}

		// class/accel setup
		sysfsAccelClassDir := path.Join(sysfsRoot, "class/accel")
		if err := os.MkdirAll(sysfsAccelClassDir, 0755); err != nil {
			return fmt.Errorf("creating fake sysfs, err: %v", err)
		}

		// links setup
		accelDirDeviceFile := path.Join(sysfsAccelClassDir, deviceName)
		accelDirControlDeviceFile := path.Join(sysfsAccelClassDir, controlDeviceName)

		if err := os.Symlink(fmt.Sprintf("../../devices/virtual/accel/%v", deviceName), accelDirDeviceFile); err != nil {
			return fmt.Errorf("creating fake sysfs, err: %v", err)
		}

		if err := os.Symlink(fmt.Sprintf("../../devices/virtual/accel/%v", controlDeviceName), accelDirControlDeviceFile); err != nil {
			return fmt.Errorf("creating fake sysfs, err: %v", err)
		}

		if err := fakeGaudiDevfs(devfsRoot, gaudi, deviceName, controlDeviceName); err != nil {
			return err
		}
	}

	return nil
}

func fakeGaudiDevfs(devfsRoot string, gaudi *device.DeviceInfo, deviceName string, controlDeviceName string) error {
	accelDevPath := path.Join(devfsRoot, "accel")
	if err := os.MkdirAll(accelDevPath, 0755); err != nil {
		return fmt.Errorf("creating fake devs, err: %v", err)
	}
	if err := createDevice(path.Join(accelDevPath, deviceName)); err != nil {
		return fmt.Errorf("creating fake devfs, err: %v", err)
	}
	if err := createDevice(path.Join(accelDevPath, controlDeviceName)); err != nil {
		return fmt.Errorf("creating fake devfs, err: %v", err)
	}

	if err := createDevice(path.Join(devfsRoot, fmt.Sprintf("hl%d", gaudi.DeviceIdx))); err != nil {
		return fmt.Errorf("creating fake devfs, err: %v", err)
	}
	if err := createDevice(path.Join(devfsRoot, fmt.Sprintf("hl_controlD%d", gaudi.DeviceIdx))); err != nil {
		return fmt.Errorf("creating fake devfs, err: %v", err)
	}

	return nil
}

func createDevice(filepath string) error {
	mode := uint32(0644 | devNullType)
	devid := int(unix.Mkdev(uint32(devNullMajor), uint32(devNullMinor)))

	if err := unix.Mknod(filepath, mode, devid); err != nil {
		return fmt.Errorf("NULL device (%d:%d) node creation failed for '%s': %w",
			devNullMajor, devNullMinor, filepath, err)
	}

	return nil
}
