package heartbeat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/byuoitav/state-parsing/alerts"
	"github.com/byuoitav/state-parsing/alerts/base"
	"github.com/byuoitav/state-parsing/alerts/device"
	"github.com/byuoitav/state-parsing/logger"
	"github.com/byuoitav/state-parsing/tasks/names"
)

const DeviceIndex = "oit-static-av-devices"

type LostHeartbeatAlertFactory struct {
	alerts.AlertFactory
}

func (h *LostHeartbeatAlertFactory) Init() {
	h.Name = names.LOST_HEARTBEAT
	h.LogLevel = logger.VERBOSE
}

func (h *LostHeartbeatAlertFactory) Run() error {
	h.I("Starting run")

	addr := fmt.Sprintf("%s/%s/_search", os.Getenv("ELK_ADDR"), DeviceIndex)

	respCode, body, err := base.MakeELKRequest(addr, "POST", []byte(HeartbeatLostQuery), h.LogLevel)
	if err != nil {
		h.E("error with the initial query: %s", err)
		return err
	}
	if respCode/100 != 2 {
		msg := fmt.Sprintf("[lost-heartbeat] Non 200 response received from the initial query: %v, %s", respCode, body)
		h.E(msg)
		return errors.New(msg)
	}
	hrresp := device.HeartbeatLostQueryResponse{}

	err = json.Unmarshal(body, &hrresp)
	if err != nil {
		h.E("couldn't unmarshal response: %s", err)
		return err
	}

	//process the alerts
	h.AlertsToSend, err = processHeartbeatLostResponse(hrresp)
	return err
}

type RestoredHeartbeatAlertFactory struct {
	alerts.AlertFactory
}

func (h *RestoredHeartbeatAlertFactory) Init() {
	h.Name = names.HEARTBEAT_RESTORED
	h.LogLevel = logger.VERBOSE
}

func (h *RestoredHeartbeatAlertFactory) Run() error {
	h.I("Starting run")

	addr := fmt.Sprintf("%s/%s/_search", os.Getenv("ELK_ADDR"), DeviceIndex)

	respCode, body, err := base.MakeELKRequest(addr, "POST", []byte(HeartbeatRestoredQuery), h.LogLevel)
	if err != nil {
		h.E("error with initial query: %s", err)
		return err
	}

	if respCode/100 != 2 {
		msg := fmt.Sprintf("[restored-heartbeat] Non 200 response received from the initial query: %v, %s", respCode, body)
		h.E(msg)
		return errors.New(msg)
	}

	hrresp := device.HeartbeatRestoredQueryResponse{}

	err = json.Unmarshal(body, &hrresp)
	if err != nil {
		h.E("couldn't unmarshal response: %s", err)
		return err
	}

	// process the alerts
	h.AlertsToSend, err = processHeartbeatRestoredResponse(hrresp)
	return err
}
