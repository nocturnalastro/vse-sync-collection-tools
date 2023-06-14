// SPDX-License-Identifier: GPL-2.0-or-later

package collectors

import (
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/callbacks"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/collectors/devices"
	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/utils"
)

var (
	GPSCollectorName = "GPS-UBX"
	GPSNavKey        = "gpsNav"
	ubxCollectables  = [1]string{GPSNavKey}
	GPSContainer     = "gpsd"
)

type GPSCollector struct {
	callback      callbacks.Callback
	DataTypes     [1]string
	interfaceName string
	ctx           clients.ContainerContext
	runningPolls  utils.WaitGroupCount
	count         int32
	running       bool
}

func (gps *GPSCollector) GetRunningPollsWG() *utils.WaitGroupCount {
	return &gps.runningPolls
}

// Start will add the key to the running pieces of data
// to be collects when polled
func (gps *GPSCollector) Start(key string) error {
	switch key {
	case All, GPSNavKey:
		gps.running = true
	default:
		return fmt.Errorf("key %s is not a colletable of %T", key, gps)
	}
	return nil
}

func (gps *GPSCollector) GetPollCount() int {
	return int(atomic.LoadInt32(&gps.count))
}

// Poll collects information from the cluster then
// calls the callback.Call to allow that to persist it
func (gps *GPSCollector) Poll(resultsChan chan PollResult) {
	gps.runningPolls.Add(1)
	defer gps.runningPolls.Done()

	gpsNav, err := devices.GetGPSNav(gps.ctx)

	if err != nil {
		resultsChan <- PollResult{
			CollectorName: GPSCollectorName,
			Errors:        []error{err},
		}
		return
	}

	line, err := json.Marshal(gpsNav)
	if err != nil {
		resultsChan <- PollResult{
			CollectorName: GPSCollectorName,
			Errors:        []error{err},
		}
		return
	} else {
		err = gps.callback.Call(fmt.Sprintf("%T", gps), GPSNavKey, string(line))
		if err != nil {
			resultsChan <- PollResult{
				CollectorName: GPSCollectorName,
				Errors:        []error{err},
			}
			return
		}
	}

	atomic.AddInt32(&gps.count, 1)
	resultsChan <- PollResult{
		CollectorName: GPSCollectorName,
		Errors:        []error{},
	}
}

// CleanUp stops a running collector
func (gps *GPSCollector) CleanUp(key string) error {
	switch key {
	case All, GPSNavKey:
		gps.running = false
	default:
		return fmt.Errorf("key %s is not a colletable of %T", key, gps)
	}
	return nil
}

// Returns a new PTPCollector from the CollectionConstuctor Factory
// It will set the lastPoll one polling time in the past such that the initial
// request to ShouldPoll should return True
func (constuctor *CollectionConstuctor) NewGPSCollector() (*GPSCollector, error) {
	ctx, err := clients.NewContainerContext(constuctor.Clientset, PTPNamespace, PodNamePrefix, GPSContainer)
	if err != nil {
		return &GPSCollector{}, fmt.Errorf("could not create container context %w", err)
	}

	collector := GPSCollector{
		interfaceName: constuctor.PTPInterface,
		ctx:           ctx,
		DataTypes:     ubxCollectables,
		running:       false,
		callback:      constuctor.Callback,
	}

	return &collector, nil
}
