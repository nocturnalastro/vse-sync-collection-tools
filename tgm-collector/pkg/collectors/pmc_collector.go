// SPDX-License-Identifier: GPL-2.0-or-later

package collectors //nolint:dupl // new collector

import (
	"fmt"

	"github.com/redhat-partner-solutions/vse-sync-collection-tools/collector-framework/pkg/clients"
	collectorsBase "github.com/redhat-partner-solutions/vse-sync-collection-tools/collector-framework/pkg/collectors"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/collector-framework/pkg/utils"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/tgm-collector/pkg/collectors/contexts"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/tgm-collector/pkg/collectors/devices"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/tgm-collector/pkg/validations"
)

const (
	PMCCollectorName = "PMC"
	PMCInfo          = "pmc-info"
)

type PMCCollector struct {
	*collectorsBase.ExecCollector
}

func (pmc *PMCCollector) poll() error {
	gmSetting, err := devices.GetPMC(pmc.GetContext())
	if err != nil {
		return fmt.Errorf("failed to fetch  %s %w", PMCInfo, err)
	}
	err = pmc.Callback.Call(&gmSetting, PMCInfo)
	if err != nil {
		return fmt.Errorf("callback failed %w", err)
	}
	return nil
}

// Poll collects information from the cluster then
// calls the callback.Call to allow that to persist it
func (pmc *PMCCollector) Poll(resultsChan chan collectorsBase.PollResult, wg *utils.WaitGroupCount) {
	defer func() {
		wg.Done()
	}()

	errorsToReturn := make([]error, 0)
	err := pmc.poll()
	if err != nil {
		errorsToReturn = append(errorsToReturn, err)
	}
	resultsChan <- collectorsBase.PollResult{
		CollectorName: PMCCollectorName,
		Errors:        errorsToReturn,
	}
}

// Returns a new PMCCollector based on values in the CollectionConstructor
func NewPMCCollector(constructor *collectorsBase.CollectionConstructor) (collectorsBase.Collector, error) {
	ctx, err := contexts.GetPTPDaemonContextOrLocal(constructor.Clientset)
	if err != nil {
		return &PMCCollector{}, fmt.Errorf("failed to create PMCCollector: %w", err)
	}

	var configFile string
	switch constructor.Target {
	case clients.TargetOCP:
		configFile = "/var/run/ptp4l.0.config"
	case clients.TargetLocal:
		configFile = "/env/ptp4l.config"
	}

	err = devices.BuildPMCFetcher(configFile)
	if err != nil {
		return &PMCCollector{}, fmt.Errorf("failed to create PMCCollector's fetcher: %w", err)
	}

	collector := PMCCollector{
		ExecCollector: collectorsBase.NewExecCollector(
			constructor.PollInterval,
			false,
			constructor.Callback,
			ctx,
		),
	}

	return &collector, nil
}

func init() {
	collectorsBase.RegisterCollector(
		PMCCollectorName,
		NewPMCCollector,
		collectorsBase.Optional,
		collectorsBase.RunOnOCP,
		[]string{
			validations.DeviceDetailsID,
			validations.DeviceDriverVersionID,
			validations.DeviceFirmwareID,
		},
	)
}
