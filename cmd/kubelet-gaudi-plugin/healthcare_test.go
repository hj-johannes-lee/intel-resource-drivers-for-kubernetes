/*
 * Copyright (c) 2024, Intel Corporation. All Rights Reserved.
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

package main

import (
	"context"
	"testing"
	"time"

	hlml "github.com/HabanaAI/gohlml"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakehlml"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakesysfs"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/device"
	helpers "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/plugintesthelpers"
)

// Event-based tests, assuming the init succeeds.
func TestUpdateHealth(t *testing.T) {
	tests := []struct {
		name                  string
		fakeEvents            []string // serial numbers
		expectedUnhealthyUIDs []string // UIDs
	}{
		{
			name:                  "HLML successfully sets single device unhealthy",
			fakeEvents:            []string{"000002"},
			expectedUnhealthyUIDs: []string{"0000-af-00-0-0x1020"},
		},
		/*		{
					name: "HLML init fails before ResourceSlice is published",
					flowControl: map[int]int{
						fakehlml.FAKE_INIT_WITH_FLAGS: fakehlml.HLML_ERROR_UNKNOWN,
					},
				},
				{
					name: "initHLML fails on DeviceCount",
					flowControl: map[int]int{
						fakehlml.FAKE_DEVICE_GET_COUNT: fakehlml.HLML_ERROR_UNKNOWN,
					},
				},
				{
					name:                  "HLML events subscription fails and all devices go unhealthy",
					expectedUnhealthyUIDs: []string{"0000-af-00-0-0x1020", "0000-b3-00-0-0x1020"},
					flowControl: map[int]int{
						fakehlml.FAKE_INIT: fakehlml.HLML_ERROR_UNKNOWN,
					},
				},*/
	}

	for _, testcase := range tests {
		t.Logf("\nTEST: %s\n", testcase.name)

		testDirs, err := helpers.NewTestDirs(device.DriverName)
		defer helpers.CleanupTest(t, testcase.name, testDirs.TestRoot)
		if err != nil {
			t.Errorf("%v: setup error: %v", testcase.name, err)
			return
		}

		testDevices := device.DevicesInfo{
			"0000-b3-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:b3:00.0", DeviceIdx: 0, UID: "0000-b3-00-0-0x1020", Serial: "000001"},
			"0000-af-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:af:00.0", DeviceIdx: 1, UID: "0000-af-00-0-0x1020", Serial: "000002"},
		}

		if err := fakesysfs.FakeSysFsGaudiContents(
			testDirs.SysfsRoot,
			testDirs.DevfsRoot,
			testDevices,
			false,
		); err != nil {
			t.Errorf("setup error: could not create fake sysfs: %v", err)
			return
		}

		fakehlml.AddDevices(testDevices)

		driver, driverErr := getFakeDriver(testDirs, WithHealthcare)
		if driverErr != nil {
			t.Errorf("%s: could not create kubelet-plugin: %v\n", testcase.name, driverErr)
			fakehlml.Reset()
			continue
		}

		if len(testcase.fakeEvents) > 0 {
			for _, serial := range testcase.fakeEvents {
				fakehlml.AddCriticalEvent(serial)
			}
			// 2 seconds per event
			totalDelay := time.Duration(2*len(testcase.fakeEvents)) * time.Second
			time.Sleep(totalDelay)
		}

		if len(testcase.expectedUnhealthyUIDs) > 0 {
			allocatable, ok := driver.state.Allocatable.(map[string]*device.DeviceInfo)
			if !ok {
				t.Error("could not cast allocatable")
			} else {
				for _, uid := range testcase.expectedUnhealthyUIDs {
					device, found := allocatable[uid]
					if !found {
						t.Errorf("unexpected result: could not find allocatable device %s", uid)
					} else if device.Healthy {
						t.Errorf("unexpected result: %s: device %s should have been unhealthy by now", testcase.name, uid)
					}
				}
			}
		}
		t.Log("shutting down test")
		// Let health monitoring go routines know they can stop.
		if err := driver.Shutdown(context.TODO()); err != nil {
			t.Errorf("could not shutdown driver: %v\n", err)
		}
		fakehlml.Reset()
		time.Sleep(2 * time.Second)
	}
}

/*
func init_should_pass(signals map[int]int) bool {
	if len(signals) {
		steps := []int{
			fakehlml.FAKE_INIT,
			fakehlml.FAKE_DEVICE_GET_HANDLE_BY_INDEX,
			fakehlml.FAKE_DEVICE_GET_SERIAL,
			fakehlml.FAKE_DEVICE_GET_PCI_INFO,

		}
		for _, step := range steps {
			if _, found := signals[]; found {
				return false
			}
		}
	}

	return true
}
*/

func TestInitHLMLErrors(t *testing.T) {
	tests := []struct {
		name              string
		expectedErr       string
		flowControl       map[uint32]uint32
		unexpectedDevices device.DevicesInfo
	}{

		{
			name: "hlml.InitWithLogs fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeInitWithFlags: fakehlml.HLMLErrorUnknown,
			},
			expectedErr: "failed to initialize HLML: unknown error",
		},
		{
			name: "hlml.DeviceCount fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeDeviceGetCount: fakehlml.HLMLErrorUnknown,
			},
			expectedErr: "failed to get device count: unknown error",
		},
		{
			name: "hlml.DeviceHandleByIndex fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeDeviceGetHandleByIndex: fakehlml.HLMLErrorUnknown,
			},
			expectedErr: "failed to get device at index 0: unknown error",
		},
		{
			name: "hlmlDevice.SerialNumber fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeDeviceGetSerial: fakehlml.HLMLErrorUnknown,
			},
			expectedErr: "failed to get serial number of device at index 0: unknown error",
		},
		{
			name: "hlmlDevice.PCIBusID fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeDeviceGetPCIInfo: fakehlml.HLMLErrorUnknown,
			},
			expectedErr: "failed to get PCI bus ID of device at index 0: unknown error",
		},
		{
			name:        "hlmlDevice.PCIBusID fails",
			flowControl: map[uint32]uint32{},
			expectedErr: "could not find device with UID 0000-d5-00-0-0x1020",
			unexpectedDevices: device.DevicesInfo{
				"0000-d5-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:d5:00.0", DeviceIdx: 2, UID: "0000-d5-00-0-0x1020", Serial: "000003"},
			},
		},
	}

	// One test setup for all cases.
	testDirs, err := helpers.NewTestDirs(device.DriverName)
	defer helpers.CleanupTest(t, "TestInitHLMLErrors", testDirs.TestRoot)
	if err != nil {
		t.Errorf("%v: setup error: %v", "TestInitHLMLErrors", err)
		return
	}

	testDevices := device.DevicesInfo{
		"0000-b3-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:b3:00.0", DeviceIdx: 0, UID: "0000-b3-00-0-0x1020", Serial: "000001"},
		"0000-af-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:af:00.0", DeviceIdx: 1, UID: "0000-af-00-0-0x1020", Serial: "000002"},
	}

	if err := fakesysfs.FakeSysFsGaudiContents(
		testDirs.SysfsRoot,
		testDirs.DevfsRoot,
		testDevices,
		false,
	); err != nil {
		t.Errorf("setup error: could not create fake sysfs: %v", err)
		return
	}

	// Start driver without health monitoring so we can break it at any point.
	driver, driverErr := getFakeDriver(testDirs, NoHealthcare)
	if driverErr != nil {
		t.Errorf("could not create kubelet-plugin: %v\n", driverErr)
		return
	}

	for _, testcase := range tests {
		t.Logf("\nTEST: %s\n", testcase.name)

		fakehlml.AddDevices(testDevices)
		if len(testcase.unexpectedDevices) > 0 {
			fakehlml.AddDevices(testcase.unexpectedDevices)
		}

		for call, ret := range testcase.flowControl {
			fakehlml.SetReturnCode(call, ret)
		}

		if err := driver.initHLML(context.TODO()); err == nil || err.Error() != testcase.expectedErr {
			t.Errorf("Unexpected return: %s, expected: %s", err, testcase.expectedErr)
		}

		fakehlml.Reset()
	}

	t.Log("shutting down test")
	// Let health monitoring go routines know they can stop.
	if err := driver.Shutdown(context.TODO()); err != nil {
		t.Errorf("could not shutdown driver: %v\n", err)
	}
}

func TestTimedHLMLEventCheckErrors(t *testing.T) {
	tests := []struct {
		name              string
		expectedRet       bool
		expectedUIDs      []string
		flowControl       map[uint32]uint32
		unexpectedDevices device.DevicesInfo
		fakeEvents        []string // serial numbers
	}{
		{
			name: "HLML WaitForEvent fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeEventSetWait: fakehlml.HLMLErrorUnknown,
			},
			expectedRet:  false,
			expectedUIDs: []string{},
		},
		{
			name: "HLML DeviceHandleBySerial fails",
			flowControl: map[uint32]uint32{
				// this will fail DeviceHandleBySerial, which in fact loops through devices
				// by index, looking for serial.
				fakehlml.FakeDeviceGetHandleByIndex: fakehlml.HLMLErrorUnknown,
			},
			expectedRet:  true,
			expectedUIDs: []string{"0000-b3-00-0-0x1020", "0000-af-00-0-0x1020"},
			fakeEvents:   []string{"000002"},
		},
		{
			name: "HLML SerialNumber fails",
			flowControl: map[uint32]uint32{
				fakehlml.FakeDeviceGetSerial: fakehlml.HLMLErrorUnknown,
			},
			expectedRet:  true,
			expectedUIDs: []string{"0000-b3-00-0-0x1020", "0000-af-00-0-0x1020"},
			fakeEvents:   []string{"000002"},
		},
		{
			name:         "unexpected device has critical event",
			expectedRet:  true,
			expectedUIDs: []string{"0000-b3-00-0-0x1020", "0000-af-00-0-0x1020"},
			fakeEvents:   []string{"000003"},
			unexpectedDevices: device.DevicesInfo{
				"0000-d5-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:d5:00.0", DeviceIdx: 2, UID: "0000-d5-00-0-0x1020", Serial: "000003"},
			},
		},
	}
	// One test setup for all cases.
	testDirs, err := helpers.NewTestDirs(device.DriverName)
	defer helpers.CleanupTest(t, "TestInitHLMLErrors", testDirs.TestRoot)
	if err != nil {
		t.Errorf("%v: setup error: %v", "TestInitHLMLErrors", err)
		return
	}

	testDevices := device.DevicesInfo{
		"0000-b3-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:b3:00.0", DeviceIdx: 0, UID: "0000-b3-00-0-0x1020", Serial: "000001"},
		"0000-af-00-0-0x1020": {Model: "0x1020", PCIAddress: "0000:af:00.0", DeviceIdx: 1, UID: "0000-af-00-0-0x1020", Serial: "000002"},
	}

	if err := fakesysfs.FakeSysFsGaudiContents(
		testDirs.SysfsRoot,
		testDirs.DevfsRoot,
		testDevices,
		false,
	); err != nil {
		t.Errorf("setup error: could not create fake sysfs: %v", err)
		return
	}

	// start driver without health monitoring so we can break it at any point
	gaudiDriver, driverErr := getFakeDriver(testDirs, NoHealthcare)
	if driverErr != nil {
		t.Errorf("could not create kubelet-plugin: %v\n", driverErr)
		return
	}

	// WithHealthcare flag normally would make driver init populate driver.state.Allocatable[].serial
	// but since we don't call HLML init, we need to populate it manually.
	allocatable, _ := gaudiDriver.state.Allocatable.(map[string]*device.DeviceInfo)
	for uid, device := range testDevices {
		allocatable[uid].Serial = device.Serial
	}

	// loop through cases
	for _, testcase := range tests {
		t.Logf("\nTEST: %s\n", testcase.name)

		// Initialize needed because driver is not calling it, and driver not created for every testcase.
		hlml.Initialize()
		fakehlml.AddDevices(testDevices)
		if len(testcase.unexpectedDevices) > 0 {
			fakehlml.AddDevices(testcase.unexpectedDevices)
		}

		registeredEventSet, err := newTestEventSet(gaudiDriver, testcase.unexpectedDevices)
		if err != nil {
			t.Errorf("could not create event set: %v", err)
			hlml.DeleteEventSet(registeredEventSet)
			fakehlml.Reset()
			continue
		}

		for call, ret := range testcase.flowControl {
			fakehlml.SetReturnCode(call, ret)
		}

		for _, serial := range testcase.fakeEvents {
			fakehlml.AddCriticalEvent(serial)
		}

		ret, uids := gaudiDriver.timedHLMLEventCheck(registeredEventSet)
		if ret != testcase.expectedRet || len(uids) != len(testcase.expectedUIDs) {
			t.Errorf("%s: unexpected return: %v, expected: %v, unexpected UIDs: %v, expected: %v", testcase.name, ret, testcase.expectedRet, uids, testcase.expectedUIDs)
		}

		hlml.DeleteEventSet(registeredEventSet)
		fakehlml.Reset()
	}

	t.Log("shutting down test")
	// Let health monitoring go routines know they can stop.
	if err := gaudiDriver.Shutdown(context.TODO()); err != nil {
		t.Errorf("could not shutdown driver: %v\n", err)
	}
}

func newTestEventSet(gaudiDriver *driver, unexpectedDevices device.DevicesInfo) (hlml.EventSet, error) {
	eventSet := hlml.NewEventSet()

	allocatable, _ := gaudiDriver.state.Allocatable.(map[string]*device.DeviceInfo)

	for _, d := range allocatable {
		err := hlml.RegisterEventForDevice(eventSet, hlml.HlmlCriticalError, d.Serial)
		if err != nil {
			return eventSet, err
		}
	}

	for _, d := range unexpectedDevices {
		err := hlml.RegisterEventForDevice(eventSet, hlml.HlmlCriticalError, d.Serial)
		if err != nil {
			return eventSet, err
		}
	}

	return eventSet, nil
}
