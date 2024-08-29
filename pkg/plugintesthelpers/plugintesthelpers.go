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

package plugintesthelpers

import (
	"fmt"
	"os"
	"path"
	"testing"
)

const (
	testRootPrefix = "test-*"
)

type TestDirsType struct {
	TestRoot         string
	CdiRoot          string
	DriverPluginRoot string
	SysfsRoot        string
	DevfsRoot        string
}

// NewTestDirs creates fake CDI root, sysfs, driverPlugin dirs and returns
// them as a testDirsType or an error.
func NewTestDirs() (TestDirsType, error) {
	testRoot, err := os.MkdirTemp("", testRootPrefix)
	if err != nil {
		return TestDirsType{}, fmt.Errorf("failed creating test root dir: %v", err)
	}

	if err := os.Chmod(testRoot, 0755); err != nil {
		return TestDirsType{}, fmt.Errorf("failed changing permissions to test root dir: %v", err)
	}

	cdiRoot := path.Join(testRoot, "cdi")
	if err := os.MkdirAll(cdiRoot, 0755); err != nil {
		return TestDirsType{}, fmt.Errorf("failed creating fake CDI root dir: %v", err)
	}

	fakeSysfsRoot := path.Join(testRoot, "sysfs")
	if err := os.MkdirAll(fakeSysfsRoot, 0755); err != nil {
		return TestDirsType{}, fmt.Errorf("failed creating fake sysfs root dir: %v", err)
	}

	driverPluginRoot := path.Join(testRoot, "kubelet-plugin")
	if err := os.MkdirAll(driverPluginRoot, 0755); err != nil {
		return TestDirsType{}, fmt.Errorf("failed creating fake driver plugin dir: %v", err)
	}

	devfsRoot := path.Join(testRoot, "dev")
	if err := os.MkdirAll(devfsRoot, 0755); err != nil {
		return TestDirsType{}, fmt.Errorf("failed creating fake devfs dir: %v", err)
	}

	return TestDirsType{
		TestRoot:         testRoot,
		CdiRoot:          cdiRoot,
		SysfsRoot:        fakeSysfsRoot,
		DriverPluginRoot: driverPluginRoot,
		DevfsRoot:        devfsRoot,
	}, nil
}

func CleanupTest(t *testing.T, testname string, testRoot string) {
	if err := os.RemoveAll(testRoot); err != nil {
		t.Logf("%v: could not cleanup temp directory %v: %v", testname, testRoot, err)
	}
}
