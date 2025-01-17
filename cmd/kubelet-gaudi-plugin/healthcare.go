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

package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	hlml "github.com/HabanaAI/gohlml"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
)

const (
	healthCheckIntervalSeconds = int(10)
)

// monitorHealth spawns a single Go routine to watch for events that
// might signal about device becoming unusable. If such an event
// happens, the ResourceSlice will be updated with kubernetes.io/healthy
// attribute set false.
// See https://github.com/kubernetes/kubernetes/issues/128979
func (d *driver) monitorHealth(ctx context.Context) {
	hlmlSyncOK := true
	ret := hlml.InitWithLogs()
	if ret != nil {
		klog.Errorf("failed to initialize HLML: %v", ret)
		return
	}

	count, ret := hlml.DeviceCount()
	if ret != nil {
		klog.Errorf("failed to get device count: %v", ret)
		return
	}

	for i := uint(0); i < count; i++ {
		hlmlDevice, ret := hlml.DeviceHandleByIndex(i)
		if ret != nil {
			klog.Errorf("failed to get device at index %d: %v", i, ret)
			continue
		}

		modId, ret := hlmlDevice.ModuleID()
		if ret != nil {
			klog.Errorf("failed to get module ID of device at index %d: %v", i, ret)
			continue
		}

		uuid, ret := hlmlDevice.UUID()
		if ret != nil {
			klog.Errorf("failed to get uuid of device at index %d: %v", i, ret)
			continue
		}

		serial, err := hlmlDevice.SerialNumber()
		if err != nil {
			klog.Errorf("failed to get serial number of device at index %d: %v", i, ret)
			continue
		}

		name, ret := hlmlDevice.Name()
		if ret != nil {
			klog.Errorf("failed to get name of device at index %d: %v", i, ret)
			continue
		}

		pciBus, ret := hlmlDevice.PCIBusID()
		if ret != nil {
			klog.Errorf("failed to get PCI bus ID of device at index %d: %v", i, ret)
			continue
		}

		pciIdHex, ret := hlmlDevice.PCIID()
		if ret != nil {
			klog.Errorf("failed to get PCI ID of device at index %d: %v", i, ret)
			continue
		}
		pciId := fmt.Sprintf("%x", pciIdHex)

		klog.V(5).Infof("Found %v, module %v, uuid %v, serial %v, PCI bus %v, PCI ID %v\n", name, modId, uuid, serial, pciBus, pciId)

		// hlml.Device.PCIID has both vendor and device ID
		uid := device.DeviceUIDFromPCIinfo(pciBus, fmt.Sprintf("0x%v", pciId[4:]))
		if gaudi, found := d.state.allocatable[uid]; found {
			klog.V(5).Infof("Saving serial %v for device %v", serial, uid)
			gaudi.Serial = serial
		} else {
			klog.V(5).Infof("Could not find device with UID %v", uid)
			hlmlSyncOK = false
		}
	}

	if !hlmlSyncOK {
		return
	}

	// Watch for device UIDs to mark unhealthy.
	idsChan := make(chan string)
	hlmlContext, stopHLMLMonitor := context.WithCancel(ctx)
	go d.watchEvents(hlmlContext, healthCheckIntervalSeconds, idsChan)

	for {
		select {
		// Listen to original ctx, when driver is shutting down, stop HLML watcher.
		case <-ctx.Done():
			stopHLMLMonitor()
			return
		case unhealthyUID := <-idsChan:
			d.unhealthy(hlmlContext, unhealthyUID)
		}
	}
}

func (d *driver) unhealthy(ctx context.Context, uid string) {
	d.state.allocatable[uid].Healthy = false
	// ignore updating error
	_ = d.UpdateResourceSlice(ctx)
}

func (d *driver) watchEvents(ctx context.Context, intervalSeconds int, idsChan chan<- string) {
	eventSet := hlml.NewEventSet()
	defer hlml.DeleteEventSet(eventSet)

	for _, d := range d.state.allocatable {
		err := hlml.RegisterEventForDevice(eventSet, hlml.HlmlCriticalError, d.Serial)
		if err != nil {
			klog.Error("Failed registering critial event for device. Marking it unhealthy", "UID", d.UID, "error", err)
			idsChan <- d.UID
			continue
		}
	}

	healthCheckInterval := time.NewTicker(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-healthCheckInterval.C:
			e, err := hlml.WaitForEvent(eventSet, 1000)
			if err != nil {
				klog.Error("hlml WaitForEvent failed", "error", err.Error())
				time.Sleep(2 * time.Second)
				continue
			}

			klog.V(5).Info("hlml event received", "event", e)

			if e.Etype != hlml.HlmlCriticalError {
				continue
			}

			dev, err := hlml.DeviceHandleBySerial(e.Serial)
			if err != nil {
				klog.Error("critical: could not get device handle by serial. All devices will go unhealthy", "event", e.Etype)
				// All devices are unhealthy
				for _, d := range d.state.allocatable {
					idsChan <- d.UID
				}
				continue
			}

			serial, err := dev.SerialNumber()
			if err != nil || len(serial) == 0 {
				klog.Error("critical: could not get serial. All devices will go unhealthy", "event", e.Etype)
				// All devices are unhealthy
				for _, d := range d.state.allocatable {
					idsChan <- d.UID
				}
				continue
			}

			found := false
			for _, d := range d.state.allocatable {
				if d.Serial == serial {
					klog.Error("critical: the device is unhealthy", "xid", e.Etype, "serial", d.Serial)
					idsChan <- d.UID
					found = true
				}
			}

			if !found {
				klog.Error("critical: could not find unhealthy device by serial. All devices will go unhealthy", "serial", serial, "event", e.Etype)
				// All devices are unhealthy
				for _, d := range d.state.allocatable {
					idsChan <- d.UID
				}
			}
		}
	}
}

func (d *driver) Shutdown(ctx context.Context) error {
	d.plugin.Stop()
	if d.hlmlShutdown != nil {
		d.hlmlShutdown()
		time.Sleep(1 * time.Second)
	}

	ret := hlml.Shutdown()
	if ret != nil {
		klog.Errorf("failed to shutdown HLML: %v", ret)
	}

	return nil
}
