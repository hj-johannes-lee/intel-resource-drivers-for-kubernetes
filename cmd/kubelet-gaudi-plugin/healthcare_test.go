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

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakehlml"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakesysfs"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/device"
	helpers "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/plugintesthelpers"
)

func TestUpdateHealth(t *testing.T) {
	tests := []struct {
		name                  string
		healthy               bool
		uid                   string
		fakeEvents            []string // serial numbers
		expectedUnhealthyUIDs []string // UIDs
	}{
		{
			name:                  "HLML sets device unhealthy",
			healthy:               false,
			uid:                   "0000-af-00-0-0x1020",
			fakeEvents:            []string{"000002"},
			expectedUnhealthyUIDs: []string{"0000-af-00-0-0x1020"},
		},
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
			t.Errorf("could not create kubelet-plugin: %v\n", driverErr)
			fakehlml.Reset()
			continue
		}

		if len(testcase.fakeEvents) > 0 {
			for _, serial := range testcase.fakeEvents {
				fakehlml.AddCriticalEvent(serial)
			}
			// 2 seconds per event
			totalDelay := 2 * len(testcase.fakeEvents)
			time.Sleep(time.Duration(totalDelay) * time.Second)
		}

		if len(testcase.expectedUnhealthyUIDs) > 0 {
			allocatable, ok := driver.state.Allocatable.(map[string]*device.DeviceInfo)
			if !ok {
				t.Error("could not cast allocatable")
			} else {
				for _, uid := range testcase.expectedUnhealthyUIDs {
					device, found := allocatable[uid]
					if !found {
						t.Errorf("could not find allocatable device %s", uid)
					} else if device.Healthy {
						t.Errorf("%s: device %s should have been unhealthy by now", testcase.name, uid)
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
