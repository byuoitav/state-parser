package heartbeat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/nerr"
	"github.com/byuoitav/state-parsing/actions"
	"github.com/byuoitav/state-parsing/actions/action"
	"github.com/byuoitav/state-parsing/actions/slack"
	"github.com/byuoitav/state-parsing/elk"
	"github.com/byuoitav/state-parsing/forwarding/device"
)

type HeartbeatRestoredJob struct {
}

const (
	HEARTBEAT_RESTORED = "heartbeat-restored"

	heartbeatRestoredQuery = `
	{  "_source": [
    "hostname",
    "last-heartbeat",
	"notifications-suppressed"], 
	"query": {
    "bool": {
      "must": [
        {
          "match": {
            "_type": "control-processor"
          }
        },
        {
          "match": {
            "alerts.lost-heartbeat.alerting": true
          }
        }
      ],
      "filter": {
        "range": {
          "last-heartbeat": {
            "gte": "now-30s"
          }
        }
      }
    }
  },
  "size": 1000
  }`
)

type heartbeatRestoredQueryResponse struct {
	Took     int  `json:"took,omitempty"`
	TimedOut bool `json:"timed_out,omitempty"`
	Shards   struct {
		Total      int `json:"total,omitempty"`
		Successful int `json:"successful,omitempty"`
		Skipped    int `json:"skipped,omitempty"`
		Failed     int `json:"failed,omitempty"`
	} `json:"_shards,omitempty"`
	Hits struct {
		Total    int     `json:"total,omitempty"`
		MaxScore float64 `json:"max_score,omitempty"`
		Hits     []struct {
			Index  string           `json:"_index,omitempty"`
			Type   string           `json:"_type,omitempty"`
			ID     string           `json:"_id,omitempty"`
			Score  float64          `json:"_score,omitempty"`
			Source elk.StaticDevice `json:"_source,omitempty"`
		} `json:"hits,omitempty"`
	} `json:"hits,omitempty"`
}

func (h *HeartbeatRestoredJob) Run(context interface{}) []action.Action {
	log.L.Debugf("Starting heartbeat restored job...")

	body, err := elk.MakeELKRequest(http.MethodPost, fmt.Sprintf("/%s/_search", elk.DEVICE_INDEX), []byte(heartbeatRestoredQuery))
	if err != nil {
		log.L.Warn("failed to make elk request to run heartbeat restored job: %s", err.String())
		return []action.Action{}
	}

	var hrresp heartbeatRestoredQueryResponse
	gerr := json.Unmarshal(body, &hrresp)
	if gerr != nil {
		log.L.Warn("failed to unmarshal elk response to run heartbeat restored job: %s", gerr)
		return []action.Action{}
	}

	acts, err := h.processResponse(hrresp)
	if err != nil {
		log.L.Warn("failed to process heartbeat restored response: %s", err.String())
		return acts
	}

	log.L.Debugf("Finished heartbeat restored job.")
	return acts
}

func (h *HeartbeatRestoredJob) processResponse(resp heartbeatRestoredQueryResponse) ([]action.Action, *nerr.E) {
	roomsToCheck := make(map[string]bool)
	// devicesToUpdate :=
	deviceIDsToUpdate := []string{}
	actionsByRoom := make(map[string][]action.Action)
	toReturn := []action.Action{}

	// there are no devices that have heartbeats restored
	if len(resp.Hits.Hits) <= 0 {
		log.L.Infof("[%s] No heartbeats restored", HEARTBEAT_RESTORED)
		return toReturn, nil
	}

	// loop through all the devices that have had restored heartbeats
	// and create an alert for them
	for i := range resp.Hits.Hits {
		device := resp.Hits.Hits[i].Source

		// get building/room off of hostname
		split := strings.Split(device.Hostname, "-")
		if len(split) != 3 {
			log.L.Warnf("%s is an improper hostname. skipping it...", device.Hostname)
			continue
		}
		building := split[0]
		room := split[1]
		roomKey := building + "-" + room

		// make sure to check this room later
		roomsToCheck[roomKey] = true

		// if it's alerting, we need to set alerting to false
		deviceIDsToUpdate = append(deviceIDsToUpdate, resp.Hits.Hits[i].ID)

		// if a device's alerts aren't suppressed, create the alert
		if device.Suppress {
			continue
		}

		slackAlert := slack.SlackAlert{
			Markdown: false,
			Attachments: []slack.SlackAttachment{
				slack.SlackAttachment{
					Fallback: fmt.Sprintf("Restored Heartbeat. Device %v sent heartbeat at %v.", device.Hostname, device.LastHeartbeat),
					Title:    "Restored Heartbeat",
					Fields: []slack.SlackAlertField{
						slack.SlackAlertField{
							Title: "Device",
							Value: device.Hostname,
							Short: true,
						},
						slack.SlackAlertField{
							Title: "Received at",
							Value: device.LastHeartbeat,
							Short: true,
						},
					},
					Color: "good",
				},
			},
		}

		a := action.Action{
			Type:    actions.SLACK,
			Device:  device.Hostname,
			Content: slackAlert,
		}

		if _, ok := actionsByRoom[roomKey]; ok {
			actionsByRoom[roomKey] = append(actionsByRoom[roomKey], a)
		} else {
			actionsByRoom[roomKey] = []action.Action{a}
		}
	}

	// mark devices as not alerting
	log.L.Infof("Marking %v devices as not alerting", len(deviceIDsToUpdate))
	device.MarkDevicesAsNotAlerting(deviceIDsToUpdate)

	/* send alerts */
	// get the rooms
	rooms, err := elk.GetRoomsBulk(func(vals map[string]bool) []string {
		ret := []string{}
		for k, _ := range vals {
			ret = append(ret, k)
		}
		return ret
	}(roomsToCheck))
	if err != nil {
		return toReturn, err
	}

	// figure out if a room's alerts are suppressed
	_, suppressed := elk.AlertingSuppressedRooms(rooms)

	// send alerts to rooms that aren't suppressed
	for room, acts := range actionsByRoom {

		if v, ok := suppressed[room]; !ok || v {
			continue
		}

		for i := range acts {
			toReturn = append(toReturn, acts[i])
		}
	}

	return toReturn, nil
}