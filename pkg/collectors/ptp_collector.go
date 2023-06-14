// SPDX-License-Identifier: GPL-2.0-or-later

package collectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/callbacks"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/collectors/devices"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/utils"
)

type PTPCollector struct {
	callback      callbacks.Callback
	data          map[string]interface{}
	running       map[string]bool
	DataTypes     [2]string
	interfaceName string
	ctx           clients.ContainerContext
	runningPolls  utils.WaitGroupCount
	lock          sync.Mutex
	count         int32
}

const (
	PTPCollectorName = "PTP"

	VendorIntel = "0x8086"
	DeviceE810  = "0x1593"

	DeviceInfo = "device-info"
	DPLLInfo   = "dpll-info"
	GNSSDev    = "gnss-dev"
	All        = "all"

	PTPNamespace  = "openshift-ptp"
	PodNamePrefix = "linuxptp-daemon-"
	PTPContainer  = "linuxptp-daemon-container"
)

var ptpCollectables = [2]string{
	DeviceInfo,
	DPLLInfo,
	// GNSSDev,
}

func (ptpDev *PTPCollector) GetRunningPollsWG() *utils.WaitGroupCount {
	return &ptpDev.runningPolls
}

func (ptpDev *PTPCollector) getNotCollectableError(key string) error {
	return fmt.Errorf("key %s is not a colletable of %T", key, ptpDev)
}

func (ptpDev *PTPCollector) getErrorIfNotCollectable(key string) error {
	for _, dataType := range ptpDev.DataTypes[:] {
		if dataType == key {
			return nil
		}
	}
	return ptpDev.getNotCollectableError(key)
}

// Start will add the key to the running pieces of data
// to be collects when polled
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

func (ptpDev *PTPCollector) GetPollCount() int {
	return int(atomic.LoadInt32(&ptpDev.count))
}

// fetchLine will call the requested key's function
// store the result of that function into the collectors data
// and returns a json encoded version of that data
func (ptpDev *PTPCollector) fetchLine(key string) (line []byte, err error) { //nolint:funlen // allow slightly long function
	ptpDev.lock.Lock()
	defer ptpDev.lock.Unlock()
	switch key {
	case DeviceInfo:
		ptpDevInfo, fetchError := devices.GetPTPDeviceInfo(ptpDev.interfaceName, ptpDev.ctx)
		if fetchError != nil {
			return []byte{}, fmt.Errorf("failed to fetch ptpDevInfo %w", fetchError)
		}
		ptpDev.data[DeviceInfo] = ptpDevInfo
		line, err = json.Marshal(ptpDevInfo)
	case DPLLInfo:
		dpllInfo, fetchError := devices.GetDevDPLLInfo(ptpDev.ctx, ptpDev.interfaceName)
		if fetchError != nil {
			return []byte{}, fmt.Errorf("failed to fetch dpllInfo %w", fetchError)
		}
		ptpDev.data[DPLLInfo] = dpllInfo
		line, err = json.Marshal(dpllInfo)
	case GNSSDev:
		// TODO make lines and timeout configs
		devInfo, ok := ptpDev.data[DeviceInfo].(devices.PTPDeviceInfo)
		if !ok {
			return []byte{}, fmt.Errorf("not able to unpack DeviceInfo %w", err)
		}
		gnssDevLine, fetchError := devices.ReadGNSSDev(ptpDev.ctx, devInfo, 1, 1)
		if fetchError != nil {
			return []byte{}, fmt.Errorf("failed to fetch gnssDevLine %w", fetchError)
		}
		ptpDev.data[GNSSDev] = gnssDevLine
		line, err = json.Marshal(gnssDevLine)
	default:
		return []byte{}, ptpDev.getNotCollectableError(key)
	}
	if err != nil {
		return []byte{}, fmt.Errorf("failed to marshall line(%v) in PTP collector: %w", key, err)
	}
	return line, nil
}

// Poll collects information from the cluster then
// calls the callback.Call to allow that to persist it
func (ptpDev *PTPCollector) Poll(resultsChan chan PollResult) {
	ptpDev.runningPolls.Add(1)
	defer ptpDev.runningPolls.Done()

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

	atomic.AddInt32(&ptpDev.count, 1)

	resultsChan <- PollResult{
		CollectorName: PTPCollectorName,
		Errors:        errorsToReturn,
	}
}

// CleanUp stops a running collector
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

// Returns a new PTPCollector from the CollectionConstuctor Factory
// It will set the lastPoll one polling time in the past such that the initial
// request to ShouldPoll should return True
func (constuctor *CollectionConstuctor) NewPTPCollector() (*PTPCollector, error) {
	ctx, err := clients.NewContainerContext(constuctor.Clientset, PTPNamespace, PodNamePrefix, PTPContainer)
	if err != nil {
		return &PTPCollector{}, fmt.Errorf("could not create container context %w", err)
	}

	data := make(map[string]interface{})
	running := make(map[string]bool)

	data[DeviceInfo], err = devices.GetPTPDeviceInfo(constuctor.PTPInterface, ctx)
	if err != nil {
		return &PTPCollector{}, fmt.Errorf("failed to fetch initial DeviceInfo %w", err)
	}
	data[DPLLInfo], err = devices.GetDevDPLLInfo(ctx, constuctor.PTPInterface)
	if err != nil {
		return &PTPCollector{}, fmt.Errorf("failed to fetch initial DevDPLLInfo %w", err)
	}
	ptpDevInfo, ok := data[DeviceInfo].(devices.PTPDeviceInfo)
	if !ok {
		return &PTPCollector{}, errors.New("DeviceInfo was not able to be unpacked")
	}
	if ptpDevInfo.VendorID != VendorIntel || ptpDevInfo.DeviceID != DeviceE810 {
		return &PTPCollector{}, errors.New("NIC device is not based on E810")
	}

	collector := PTPCollector{
		interfaceName: constuctor.PTPInterface,
		ctx:           ctx,
		DataTypes:     ptpCollectables,
		data:          data,
		running:       running,
		callback:      constuctor.Callback,
	}

	return &collector, nil
}
