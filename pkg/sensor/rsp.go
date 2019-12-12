/* Apache v2 license
*  Copyright (C) <2019> Intel Corporation
*
*  SPDX-License-Identifier: Apache-2.0
 */

package sensor

import (
	"github.impcloud.net/RSP-Inventory-Suite/loss-prevention-service/pkg/jsonrpc"
	"strconv"
)

const (
	DefaultFacility = "DEFAULT_FACILITY"
)

type Personality string

const (
	NoPersonality Personality = "NONE"
	Exit          Personality = "EXIT"
	POS           Personality = "POS"
	FittingRoom   Personality = "FITTING_ROOM"
)

type RSP struct {
	DeviceId     string      `json:"device_id" bson:"device_id"`
	FacilityId   string      `json:"facility_id" bson:"facility_id"`
	Personality  Personality `json:"personality" bson:"personality"`
	Aliases      []string    `json:"aliases" bson:"aliases"`
	IsInDeepScan bool        `json:"-" bson:"-"`
}

func NewRSP(deviceId string) *RSP {
	rsp := RSP{
		DeviceId:    deviceId,
		Personality: NoPersonality,
		FacilityId:  DefaultFacility,
	}
	// setup a default alias for antenna 0
	rsp.Aliases = []string{rsp.AntennaAlias(0)}
	return &rsp
}

func NewRSPFromConfigNotification(notification *jsonrpc.SensorConfigNotification) *RSP {
	cfg := notification.Params
	return &RSP{
		DeviceId:    cfg.DeviceId,
		FacilityId:  cfg.FacilityId,
		Personality: Personality(cfg.Personality),
		Aliases:     cfg.Aliases,
	}
}

// AntennaAlias gets the string alias of an RSP based on the antenna port
// format is DeviceId-AntennaId,  ie. RSP-150009-0
// If there is an alias defined for that antenna port, use that instead
// Note that each antenna port is supposed to refer to that index in the
// rsp.Aliases slice
func (rsp *RSP) AntennaAlias(antennaId int) string {
	if len(rsp.Aliases) > antennaId {
		return rsp.Aliases[antennaId]
	} else {
		return rsp.DeviceId + "-" + strconv.Itoa(antennaId)
	}
}

// IsExitSensor returns true if this RSP has the EXIT personality
func (rsp *RSP) IsExitSensor() bool {
	return rsp.Personality == Exit
}

// IsPOSSensor returns true if this RSP has the POS personality
func (rsp *RSP) IsPOSSensor() bool {
	return rsp.Personality == POS
}
