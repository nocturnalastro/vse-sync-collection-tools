// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/callbacks"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/collectors/devices"
)

type PTPCollector struct {
	lastPoll        time.Time
	callback        callbacks.Callback
	data            map[string]interface{}
	running         map[string]bool
	DataTypes       [3]string
	ctx             clients.ContainerContext
	interfaceName   string
	inversePollRate float64
}

const (
	VendorIntel = "0x8086"
	DeviceE810  = "0x1593"

	DeivceInfo = "device-info"
	DLLInfo    = "dll-info"
	GNNSSTTY   = "gnss-tty"
	All        = "all"

	PTPNamespace  = "openshift-ptp"
	PodNamePrefix = "linuxptp-daemon-"
	PTPContainer  = "linuxptp-daemon-container"
)

var collectables = [3]string{
	DeivceInfo,
	DLLInfo,
	GNNSSTTY,
}

func NewPTPCollector(
	ptpInterface string,
	pollRate float64,
	clientset *clients.Clientset,
	callback callbacks.Callback,
) (PTPCollector, error) {
	ctx, err := clients.NewContainerContext(clientset, PTPNamespace, PodNamePrefix, PTPContainer)
	if err != nil {
		return PTPCollector{}, fmt.Errorf("could not create container context %w", err)
	}

	data := make(map[string]interface{})
	running := make(map[string]bool)

	data[DeivceInfo] = devices.GetPTPDeviceInfo(ptpInterface, ctx)
	data[DLLInfo] = devices.GetDevDPLLInfo(ctx, ptpInterface)

	ptpDevInfo, ok := data[DeivceInfo].(devices.PTPDeviceInfo)
	if !ok {
		return PTPCollector{}, fmt.Errorf("DeviceInfo was not able to be unpacked")
	}
	if ptpDevInfo.VendorID != VendorIntel || ptpDevInfo.DeviceID != DeviceE810 {
		return PTPCollector{}, fmt.Errorf("NIC device is not based on E810")
	}

	collector := PTPCollector{
		interfaceName:   ptpInterface,
		ctx:             ctx,
		DataTypes:       collectables,
		data:            data,
		running:         running,
		callback:        callback,
		inversePollRate: 1.0 / pollRate,
		lastPoll:        time.Now(),
	}

	return collector, nil
}

func (ptpDev *PTPCollector) getNotCollectableError(key string) error {
	return fmt.Errorf("key %s is not a colletable of %T", key, ptpDev)
}

func (ptpDev *PTPCollector) getErrorIfNotCollectable(key string) error {
	if _, ok := ptpDev.data[key]; !ok {
		return ptpDev.getNotCollectableError(key)
	} else {
		return nil
	}
}

func (ptpDev *PTPCollector) Start(key string) error {
	switch key {
	case All:
		for _, dataType := range ptpDev.DataTypes[:] {
			log.Debugf("starting: %s", dataType)
			ptpDev.running[dataType] = true
		}
	default:
		err := ptpDev.getErrorIfNotCollectable(key)
		if err != nil {
			return err
		}
		ptpDev.running[key] = true
	}
	return nil
}

// Checks to see if the enou
func (ptpDev *PTPCollector) ShouldPoll() bool {
	return time.Since(ptpDev.lastPoll).Seconds() >= ptpDev.inversePollRate
}

func (ptpDev *PTPCollector) fetchLine(key string) (line []byte, err error) {
	switch key {
	case DeivceInfo:
		ptpDevInfo := devices.GetPTPDeviceInfo(ptpDev.interfaceName, ptpDev.ctx)
		ptpDev.data[DeivceInfo] = ptpDevInfo
		line, err = json.Marshal(ptpDevInfo)
	case DLLInfo:
		dllInfo := devices.GetDevDPLLInfo(ptpDev.ctx, ptpDev.interfaceName)
		ptpDev.data[DLLInfo] = dllInfo
		line, err = json.Marshal(dllInfo)
	case GNNSSTTY:
		// TODO make lines and timeout configs
		devInfo, ok := ptpDev.data[DeivceInfo].(devices.PTPDeviceInfo)
		if !ok {
			return nil, fmt.Errorf("DeviceInfo was not able to be unpacked")
		}
		gnssTTYLine := devices.ReadTtyGNSS(ptpDev.ctx, devInfo, 1, 1)

		ptpDev.data[GNNSSTTY] = gnssTTYLine
		line, err = json.Marshal(gnssTTYLine)
	default:
		return nil, ptpDev.getNotCollectableError(key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to marshall line(%v) in PTP collector: %w", key, err)
	}
	return line, nil
}

// Poll collects information from the cluster then
// calls the callback.Call to allow that to persist it
func (ptpDev *PTPCollector) Poll() []error {
	errorsToReturn := make([]error, 0)

	for key, isRunning := range ptpDev.running {
		if isRunning {
			line, err := ptpDev.fetchLine(key)
			// TODO: handle (better)
			if err != nil {
				errorsToReturn = append(errorsToReturn, err)
			} else {
				err = ptpDev.callback.Call(fmt.Sprintf("%T", ptpDev), key, string(line))
				if err != nil {
					errorsToReturn = append(errorsToReturn, err)
				}
			}
		}
	}
	return errorsToReturn
}

// Stops a running collector then do clean
func (ptpDev *PTPCollector) CleanUp(key string) error {
	switch key {
	case All:
		ptpDev.running = make(map[string]bool)
	default:
		err := ptpDev.getErrorIfNotCollectable(key)
		if err != nil {
			return err
		}
		delete(ptpDev.running, key)
	}
	return nil
}

func init() {
	Register("PTP", NewPTPCollector)
}