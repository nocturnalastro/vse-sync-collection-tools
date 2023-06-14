// SPDX-License-Identifier: GPL-2.0-or-later

package devices

import (
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-partner-solutions/vse-sync-testsuite/pkg/clients"
)

var (
	fetcherMutex sync.Mutex
)

type PTPDeviceInfo struct {
	Timestamp string `json:"date" fetcherKey:"date"`
	VendorID  string `json:"vendorId" fetcherKey:"vendorID"`
	DeviceID  string `json:"deviceInfo" fetcherKey:"devID"`
	GNSSDev   string `json:"GNSSDev" fetcherKey:"gnss"`
}

type DevDPLLInfo struct {
	Timestamp string `json:"date" fetcherKey:"date"`
	EECState  string `json:"EECState" fetcherKey:"dpll_0_state"`
	PPSState  string `json:"PPSState" fetcherKey:"dpll_1_state"`
	PPSOffset string `json:"PPSOffset" fetcherKey:"dpll_1_offset"`
}
type GNSSDevLines struct {
	Timestamp string `json:"date" fetcherKey:"date"`
	Dev       string `json:"dev"`
	Lines     string `json:"lines" fetcherKey:"lines"`
}

var (
	devFetcher  map[string]*fetcher
	gnssFetcher map[string]*fetcher
	dpllFetcher map[string]*fetcher
	dateCmd     *clients.Cmd
)

func init() {
	devFetcher = make(map[string]*fetcher)
	gnssFetcher = make(map[string]*fetcher)
	dpllFetcher = make(map[string]*fetcher)
	dateCmdInst, err := clients.NewCmd("date", "date --iso-8601=ns")
	if err != nil {
		panic(err)
	}
	dateCmd = dateCmdInst
	dateCmd.SetCleanupFunc(strings.TrimSpace)
}

func GetPTPDeviceInfo(interfaceName string, ctx clients.ContainerContext) (PTPDeviceInfo, error) {
	devInfo := PTPDeviceInfo{}
	// Find the dev for the GNSS for this interface
	fetcherInst, ok := devFetcher[interfaceName]
	if !ok {
		fetcherMutex.Lock()
		defer fetcherMutex.Unlock()
		fetcherInst = NewFetcher()
		devFetcher[interfaceName] = fetcherInst

		fetcherInst.AddCommand(dateCmd)

		err := fetcherInst.AddNewCommand(
			"gnss",
			fmt.Sprintf("ls /sys/class/net/%s/device/gnss/", interfaceName),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "gnss", err.Error())
			return devInfo, fmt.Errorf("failed to fetch devInfo %w", err)
		}

		err = fetcherInst.AddNewCommand(
			"devID",
			fmt.Sprintf("cat /sys/class/net/%s/device/device", interfaceName),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "devId", err.Error())
			return devInfo, fmt.Errorf("failed to fetch devInfo %w", err)
		}
		err = fetcherInst.AddNewCommand("vendorID",
			fmt.Sprintf("cat /sys/class/net/%s/device/vendor", interfaceName),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "vendorID", err.Error())
			return devInfo, fmt.Errorf("failed to fetch devInfo %w", err)
		}
	}

	err := fetcherInst.Fetch(ctx, &devInfo)
	if err != nil {
		log.Errorf("failed to fetch devInfo %s", err.Error())
		return devInfo, fmt.Errorf("failed to fetch devInfo %w", err)
	}
	devInfo.GNSSDev = "/dev/" + devInfo.GNSSDev
	return devInfo, nil
}

// Read lines from the GNSSDev of the passed devInfo.
func ReadGNSSDev(ctx clients.ContainerContext, devInfo PTPDeviceInfo, lines, timeoutSeconds int) (GNSSDevLines, error) {
	fetcherInst, ok := gnssFetcher[devInfo.GNSSDev]
	if !ok {
		fetcherMutex.Lock()
		defer fetcherMutex.Unlock()
		fetcherInst = NewFetcher()
		gnssFetcher[devInfo.GNSSDev] = fetcherInst

		fetcherInst.AddCommand(dateCmd)

		err := fetcherInst.AddNewCommand(
			"lines",
			fmt.Sprintf("timeout %d head -n %d %s", timeoutSeconds, lines, devInfo.GNSSDev),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "lines", err.Error())
			return GNSSDevLines{}, err
		}
	}

	gnssInfo := GNSSDevLines{
		Dev: devInfo.GNSSDev,
	}
	err := fetcherInst.Fetch(ctx, &gnssInfo)
	if err != nil {
		log.Errorf("failed to fetch gnssInfo %s", err.Error())
		return GNSSDevLines{}, err
	}
	return gnssInfo, nil
}

// GetDevDPLLInfo returns the device DPLL info for an interface.
func GetDevDPLLInfo(ctx clients.ContainerContext, interfaceName string) (DevDPLLInfo, error) {
	dpllInfo := DevDPLLInfo{}
	fetcherInst, ok := dpllFetcher[interfaceName]
	if !ok {
		fetcherMutex.Lock()
		defer fetcherMutex.Unlock()
		fetcherInst = NewFetcher()
		dpllFetcher[interfaceName] = fetcherInst

		fetcherInst.AddCommand(dateCmd)

		err := fetcherInst.AddNewCommand(
			"dpll_0_state",
			fmt.Sprintf("cat /sys/class/net/%s/device/dpll_0_state", interfaceName),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "dpll_0_state", err.Error())
			return dpllInfo, err
		}

		err = fetcherInst.AddNewCommand(
			"dpll_1_state",
			fmt.Sprintf("cat /sys/class/net/%s/device/dpll_1_state", interfaceName),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "dpll_1_state", err.Error())
			return dpllInfo, err
		}

		err = fetcherInst.AddNewCommand(
			"dpll_1_offset",
			fmt.Sprintf("cat /sys/class/net/%s/device/dpll_1_offset", interfaceName),
			true,
		)
		if err != nil {
			log.Errorf("failed to add command %s %s", "dpll_1_offset", err.Error())
			return dpllInfo, err
		}
	}
	err := fetcherInst.Fetch(ctx, &dpllInfo)
	if err != nil {
		log.Errorf("failed to fetch dpllInfo %s", err.Error())
		return dpllInfo, err
	}
	return dpllInfo, nil
}
