package managers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/byuoitav/common/log"
	"github.com/byuoitav/common/nerr"
	"github.com/byuoitav/common/v2/events"
	"github.com/byuoitav/state-parser/elk"
)

type ElkBulkUpdateItem struct {
	Header ElkUpdateHeader
	Doc    interface{}
}

type ElkUpdateHeader struct {
	Index HeaderIndex `json:"index"`
}

type HeaderIndex struct {
	Index string `json:"_index"`
	Type  string `json:"_type"`
	ID    string `json:"_id,omitempty"`
}

//there are other types, but we don't worry about them, since we don't really do any smart parsing at this time.
type BulkUpdateResponse struct {
	Errors bool `json:"errors"`
}

//NOT THREAD SAFE
type ElkTimeseriesForwarder struct {
	incomingChannel chan events.Event
	buffer          []ElkBulkUpdateItem
	ElkStaticForwarder
}

//returns a default elk event forwarder after setting it up.
func GetDefaultElkTimeSeries(URL string, index func() string) *ElkTimeseriesForwarder {
	toReturn := &ElkTimeseriesForwarder{
		incomingChannel: make(chan events.Event, 1000),
		ElkStaticForwarder: ElkStaticForwarder{
			interval: time.Second * 15,
			url:      URL,
			index:    index,
		},
	}

	//start the manager
	go toReturn.start()

	return toReturn
}

func (e *ElkTimeseriesForwarder) Send(toSend interface{}) error {

	var event events.Event

	switch e := toSend.(type) {
	case *events.Event:
		event = *e
	case events.Event:
		event = e
	default:
		return nerr.Create("Invalid type to send via an Elk Event Forwarder, must be an event from the byuoitav/common/events package.", "invalid-type")
	}

	e.incomingChannel <- event

	return nil
}

//starts the manager and buffer.
func (e *ElkTimeseriesForwarder) start() {

	log.L.Infof("Starting event forwarder for %v", e.index())
	ticker := time.NewTicker(e.interval)

	for {
		select {
		case <-ticker.C:
			//send it off
			log.L.Debugf("Sending bulk ELK update for %v", e.index())

			go forward(e.url, e.buffer)
			e.buffer = []ElkBulkUpdateItem{}

		case event := <-e.incomingChannel:
			e.bufferevent(event)
		}
	}
}

//NOT THREAD SAFE
func (e *ElkTimeseriesForwarder) bufferevent(event events.Event) {
	e.buffer = append(e.buffer, ElkBulkUpdateItem{
		Header: ElkUpdateHeader{Index: HeaderIndex{
			Index: e.index(),
			Type:  "event",
		}},
		Doc: event,
	})
}

func forward(url string, toSend []ElkBulkUpdateItem) {

	log.L.Debugf("Sending and update for %v devices.", len(toSend))

	if len(toSend) == 0 {
		log.L.Infof("No updates to send. returning.")
		return
	}

	//DEBUG
	for i := range toSend {
		log.L.Debugf("%+v", toSend[i])
	}

	log.L.Debugf("Building payload")
	//build our payload
	payload := []byte{}
	for i := range toSend {
		headerbytes, err := json.Marshal(toSend[i].Header)
		if err != nil {
			log.L.Errorf("Couldn't marshal header for elk event bulk update: %v", toSend[i])
			continue
		}
		bodybytes, err := json.Marshal(toSend[i].Doc)
		if err != nil {
			log.L.Errorf("Couldn't marshal header for elk event bulk update: %v", toSend[i])
			continue
		}
		payload = append(payload, headerbytes...)
		payload = append(payload, '\n')
		payload = append(payload, bodybytes...)
		payload = append(payload, '\n')
	}

	//once our payload is built
	log.L.Debugf("Payload built, sending...")

	url = strings.Trim(url, "/")         //remove any trailing slash so we can append it again
	addr := fmt.Sprintf("%v/_bulk", url) //make the addr

	resp, er := elk.MakeGenericELKRequest(addr, "POST", payload)
	if er != nil {
		log.L.Errorf("Couldn't send bulk update. error %v", er.Error())
		return
	}

	elkresp := BulkUpdateResponse{}

	err := json.Unmarshal(resp, &elkresp)
	if err != nil {
		log.L.Errorf("Unknown response received from ELK in response to bulk update: %s", resp)
		return
	}
	if elkresp.Errors {
		log.L.Errorf("Errors received from ELK during bulk update %v", resp)
		return
	}
	log.L.Debugf("Successfully sent bulk ELK updates")
}
