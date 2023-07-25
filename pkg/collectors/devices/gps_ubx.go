// SPDX-License-Identifier: GPL-2.0-or-later

package devices

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/redhat-partner-solutions/vse-sync-collection-tools/pkg/callbacks"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/pkg/clients"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/pkg/fetcher"
	"github.com/redhat-partner-solutions/vse-sync-collection-tools/pkg/utils"
)

type GPSDetails struct {
	NavStatus       GPSNavStatus         `json:"navStatus" fetcherKey:"navStatus"`
	FirmwareVersion string               `json:"firmwareVersion" fetcherKey:"firmwareVersion"`
	AntennaDetails  []*GPSAntennaDetails `json:"antennaDetails" fetcherKey:"antennaDetails"`
	NavClock        GPSNavClock          `json:"navClock" fetcherKey:"navClock"`
}

type GPSNavStatus struct {
	Timestamp string `json:"timestamp"`
	GPSFix    string `json:"GPSFix"`
}

type GPSNavClock struct {
	Timestamp string `json:"timestamp"`
	TimeAcc   int    `json:"timeAcc"`
	FreqAcc   int    `json:"freqAcc"`
}

type GPSAntennaDetails struct {
	Timestamp string `json:"timestamp"`
	BlockID   int    `json:"blockId"`
	Status    int    `json:"status"`
	Power     int    `json:"power"`
}

func (gpsNav *GPSDetails) GetAnalyserFormat() ([]*callbacks.AnalyserFormatType, error) {
	messages := []*callbacks.AnalyserFormatType{}
	messages = append(messages, &callbacks.AnalyserFormatType{
		ID: "gnss/time-error",
		Data: map[string]any{
			"timestamp": gpsNav.NavClock.Timestamp,
			"state":     gpsNav.NavStatus.GPSFix,
			"terror":    gpsNav.NavClock.TimeAcc,
			"ferror":    gpsNav.NavClock.FreqAcc,
		},
	})

	for _, ant := range gpsNav.AntennaDetails {
		messages = append(messages, &callbacks.AnalyserFormatType{
			ID: "gnss/rf-mon",
			Data: map[string]any{
				"timestamp": ant.Timestamp,
				"blockID":   ant.BlockID,
				"state":     ant.Status,
				"power":     ant.Power,
			},
		})
	}
	return messages, nil
}

var (
	timeStampPattern  = `(\d+.\d+)`
	ubxNavStatusRegex = regexp.MustCompile(
		timeStampPattern +
			`\nUBX-NAV-STATUS:\n\s+iTOW (\d+) gpsFix (\d) flags (.*) fixStat ` +
			`(.*) flags2\s(.*)\n\s+ttff\s(\d+), msss (\d+)\n\n`,
		// ubxtool output example:
		// 1686916187.0584
		// UBX-NAV-STATUS:
		//   iTOW 474605000 gpsFix 3 flags 0xdd fixStat 0x0 flags2 0x8
		//   ttff 25030, msss 4294967295
	)
	ubxNavClockRegex = regexp.MustCompile(
		timeStampPattern +
			`\nUBX-NAV-CLOCK:\n\s+iTOW (\d+) clkB (\d+) clkD (\d+) tAcc (\d+) fAcc (\d+)`,
		// 1686916187.0586
		// UBX-NAV-CLOCK:
		//   iTOW 474605000 clkB 61594 clkD 56 tAcc 5 fAcc 164
	)

	ubxAntFullBlockRegex = regexp.MustCompile(
		timeStampPattern +
			`\nUBX-MON-RF:\n` +
			`\s+version \d nBlocks (\d) reserved1 \d \d\n(?s:([^UBX-]*)[UBX-]?)`,
		// 1686916187.0584
		// UBX-MON-RF:
		//  version 0 nBlocks 2 reserved1 0 0
		//    blockId 0 flags x0 antStatus 2 antPower 1 postStatus 0 reserved2 0 0 0 0
		//     noisePerMS 82 agcCnt 6318 jamInd 3 ofsI 15 magI 154 ofsQ 2 magQ 145
		//     reserved3 0 0 0
		//    blockId 1 flags x0 antStatus 2 antPower 1 postStatus 0 reserved2 0 0 0 0
		//     noisePerMS 49 agcCnt 6669 jamInd 2 ofsI 11 magI 146 ofsQ 1 magQ 139
		//     reserved3 0 0 0
	)
	ubxAntInternalBlockRegex = regexp.MustCompile(
		`\s+blockId (\d) flags \w+ antStatus (\d) antPower (\d+) postStatus \d reserved2 \d \d \d \d\n` +
			`\s+noisePerMS \d+ agcCnt \d+ jamInd \d ofsI \d+ magI \d+ ofsQ \d magQ \d+\n` +
			`\s+reserved3 \d \d \d\n`,
		//    blockId 0 flags x0 antStatus 2 antPower 1 postStatus 0 reserved2 0 0 0 0
		//     noisePerMS 82 agcCnt 6318 jamInd 3 ofsI 15 magI 154 ofsQ 2 magQ 145
		//     reserved3 0 0 0
		//    blockId 1 flags x0 antStatus 2 antPower 1 postStatus 0 reserved2 0 0 0 0
		//     noisePerMS 49 agcCnt 6669 jamInd 2 ofsI 11 magI 146 ofsQ 1 magQ 139
		//     reserved3 0 0 0
	)
	ubxFirmwareVersion = regexp.MustCompile(
		timeStampPattern +
			`\nUBX-MON-VER:` +
			`\n\s+swVersion (.*)` +
			`\n\s+hwVersion (.*)` +
			`\n\s+((?:extension .*(?:\n\s+)?)+)`,
		// 1689260332.4728
		// UBX-MON-VER:
		//   swVersion EXT CORE 1.00 (3fda8e)
		//   hwVersion 00190000
		//   extension ROM BASE 0x118B2060
		//   extension FWVER=TIM 2.20
		//   extension PROTVER=29.20
		//   extension MOD=ZED-F9T
		//   extension GPS;GLO;GAL;BDS
		//   extension SBAS;QZSS
		//   extension NAVIC

	)
	fwVersionExtension = regexp.MustCompile(`extension FWVER=(.*)`)
	gpsFetcher         *fetcher.Fetcher
)

func init() {
	gpsFetcher = fetcher.NewFetcher()
	gpsFetcher.SetPostProcessor(processUBX)
	err := gpsFetcher.AddNewCommand(
		"GPS",
		"ubxtool -t -p NAV-STATUS -p NAV-CLOCK -p MON-VER -p MON-RF -P 29.20",
		true,
	)
	if err != nil {
		panic(fmt.Errorf("failed to setup GPS fetcher %w", err))
	}
}

// processUBXNavStatus parses the output of the ubxtool extracting the required values for GPSNav
func processUBXNavStatus(result map[string]string) (map[string]any, error) {
	processedResult := make(map[string]any)
	matchNav := ubxNavStatusRegex.FindStringSubmatch(result["GPS"])
	if len(matchNav) == 0 {
		return processedResult, fmt.Errorf(
			"unable to parse UBX Nav Status from %s",
			result["GPS"],
		)
	}
	timestampSatus, err := utils.ParseTimestamp(matchNav[1])
	if err != nil {
		return processedResult, fmt.Errorf("failed to parse navStatusTimestamp %w", err)
	}
	processedResult["navStatus"] = GPSNavStatus{
		Timestamp: timestampSatus.Format(time.RFC3339Nano),
		GPSFix:    matchNav[3],
	}
	return processedResult, nil
}

// processUBXNavClock parses the output of the ubxtool extracting the required values for GPSNav
func processUBXNavClock(result map[string]string) (map[string]any, error) {
	processedResult := make(map[string]any)
	matchNav := ubxNavClockRegex.FindStringSubmatch(result["GPS"])
	if len(matchNav) == 0 {
		return processedResult, fmt.Errorf(
			"unable to parse UBX Nav Status or Clock from %s",
			result["GPS"],
		)
	}
	timestampClock, err := utils.ParseTimestamp(matchNav[1])
	if err != nil {
		return processedResult, fmt.Errorf("failed to parse navClockTimestamp %w", err)
	}
	timeAcc, err := strconv.Atoi(matchNav[5])
	if err != nil {
		return processedResult, fmt.Errorf("failed to convert %s into and int", matchNav[13])
	}
	freqAcc, err := strconv.Atoi(matchNav[6])
	if err != nil {
		return processedResult, fmt.Errorf("failed to convert %s into and int", matchNav[14])
	}
	processedResult["navClock"] = GPSNavClock{
		Timestamp: timestampClock.Format(time.RFC3339Nano),
		TimeAcc:   timeAcc,
		FreqAcc:   freqAcc,
	}
	return processedResult, nil
}

func processUBXMonRF(result map[string]string) (map[string]any, error) { //nolint:funlen // allow for a slightly long function
	processedResult := make(map[string]any)

	antFullMatch := ubxAntFullBlockRegex.FindStringSubmatch(result["GPS"])
	if len(antFullMatch) == 0 {
		return processedResult, fmt.Errorf("failed to match UBX MON in %s", result["GPS"])
	}

	timestampMon, err := utils.ParseTimestamp(antFullMatch[1])
	if err != nil {
		return processedResult, fmt.Errorf("failed to parse monTimestamp %w", err)
	}
	timestamp := timestampMon.Format(time.RFC3339Nano)

	nBlocks, err := strconv.Atoi(antFullMatch[2])
	if err != nil {
		return processedResult, fmt.Errorf("failed to parse UBX antenna monitoring %w", err)
	}

	antBlockMatches := ubxAntInternalBlockRegex.FindAllStringSubmatch(antFullMatch[3], nBlocks)

	antennaDetails := make([]*GPSAntennaDetails, 0)
	for _, antBlock := range antBlockMatches {
		antBlockIDValue, err := strconv.Atoi(antBlock[1])
		if err != nil {
			return processedResult, fmt.Errorf("failed to convert %s to an int for antBlockIDValue %w", antBlock[1], err)
		}
		antStatusValue, err := strconv.Atoi(antBlock[2])
		if err != nil {
			return processedResult, fmt.Errorf("failed to convert %s to an int for antStatusValue %w", antBlock[2], err)
		}
		antPowerValue, err := strconv.Atoi(antBlock[3])
		if err != nil {
			return processedResult, fmt.Errorf("failed to convert %s to an int for antPowerValue %w", antBlock[3], err)
		}

		antennaDetails = append(antennaDetails, &GPSAntennaDetails{
			Timestamp: timestamp,
			BlockID:   antBlockIDValue,
			Status:    antStatusValue,
			Power:     antPowerValue,
		})
	}

	processedResult["antennaDetails"] = antennaDetails
	return processedResult, nil
}

func processUBXMonVer(result map[string]string) (map[string]any, error) {
	processedResult := make(map[string]any)
	match := ubxFirmwareVersion.FindStringSubmatch(result["GPS"])
	if len(match) == 0 {
		return processedResult, fmt.Errorf(
			"unable to parse UBX MON Version from %s",
			result["GPS"],
		)
	}

	timestampMonVer, err := utils.ParseTimestamp(match[1])
	if err != nil {
		return processedResult, fmt.Errorf("failed to parse versionTimestamp %w", err)
	}
	processedResult["versionTimestamp"] = timestampMonVer.Format(time.RFC3339Nano)

	version := fwVersionExtension.FindStringSubmatch(match[4])
	if len(version) == 0 {
		return processedResult, fmt.Errorf(
			"unable to parse version from extenstions in %s",
			match[4],
		)
	}
	processedResult["firmwareVersion"] = version[1]
	return processedResult, nil
}

func processUBX(result map[string]string) (map[string]any, error) { //nolint:funlen // allow slightly long function
	processedResult := make(map[string]any)
	processedUBXNavStatus, err := processUBXNavStatus(result)
	if err != nil {
		log.Errorf("processUBXNav Failed: %s", err.Error())
		return processedResult, err
	}
	for key, value := range processedUBXNavStatus {
		processedResult[key] = value
	}
	processedUBXNavClock, err := processUBXNavClock(result)
	if err != nil {
		log.Errorf("processUBXNav Failed: %s", err.Error())
		return processedResult, err
	}
	for key, value := range processedUBXNavClock {
		processedResult[key] = value
	}

	processedUBXMonRF, err := processUBXMonRF(result)
	if err != nil {
		log.Errorf("processUBXMon Failed: %s", err.Error())
		return processedResult, err
	}
	for key, value := range processedUBXMonRF {
		processedResult[key] = value
	}

	processedUBXMonVer, err := processUBXMonVer(result)
	if err != nil {
		log.Errorf("processUBXMon Failed: %s", err.Error())
		return processedResult, err
	}
	for key, value := range processedUBXMonVer {
		processedResult[key] = value
	}

	return processedResult, nil
}

// GetGPSNav returns GPSNav of the host
func GetGPSNav(ctx clients.ContainerContext) (GPSDetails, error) {
	gpsNav := GPSDetails{}
	err := gpsFetcher.Fetch(ctx, &gpsNav)
	if err != nil {
		log.Debugf("failed to fetch gpsNav %s", err.Error())
		return gpsNav, fmt.Errorf("failed to fetch gpsNav %w", err)
	}
	return gpsNav, nil
}
