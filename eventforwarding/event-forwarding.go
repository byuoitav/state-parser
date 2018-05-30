package eventforwarding

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/byuoitav/event-translator-microservice/elkreporting"
	"github.com/byuoitav/salt-translator-service/elk"
	"github.com/fatih/color"
)

var apiForwardingChannel chan elkreporting.ElkEvent
var heartbeatForwardingChannel chan elk.Event

func Init() {
	apiurl := os.Getenv("ELASTIC_API_EVENTS")
	if len(apiurl) < 1 {
		//nothing there, panic
		log.Printf("No API  endpoint specified")
		os.Exit(1)
	}
	heartbeaturl := os.Getenv("ELASTIC_HEARTBEAT_EVENTS")
	if len(heartbeaturl) < 1 {
		//nothing there, panic
		log.Printf("No HEARTBEAT endpoint specified")
		os.Exit(1)
	}

	//make our channel
	apiForwardingChannel = make(chan elkreporting.ElkEvent, 5000)
	heartbeatForwardingChannel = make(chan elk.Event, 5000)

	ForwardingWorkers := 5

	for i := 0; i < ForwardingWorkers; i++ {

		log.Printf(color.HiMagentaString("Starting forwarding worker %v", i))

		go func() {
			//we just send it up
			for {

				select {
				case e := <-apiForwardingChannel:
					//print out the event
					forwardEvent(e, apiurl)
				case e := <-heartbeatForwardingChannel:
					forwardEvent(e, heartbeaturl)
					//print out the length
					log.Printf(color.HiMagentaString("Heartbeat Forwarding Channel Size: %v.", len(heartbeatForwardingChannel)))
				}
			}
		}()
	}
}

func forwardEvent(e interface{}, url string) {
	start := time.Now()

	//	log.Printf("[forwarder] Forwarding event to %v", url)
	b, err := json.Marshal(e)
	if err != nil {
		log.Printf("[forwarder] There was a problem marshalling the event: %v", err.Error())
		return
	}
	//ship it on
	resp, err := http.Post(url, "appliciation/json", bytes.NewBuffer(b))
	if err != nil {
		log.Printf("[forwarder] There was a problem sending the event: %v", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		log.Printf("[forwarder] Non-200 response recieved: %v.", resp.StatusCode)

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[forwarder] could not read body: %v", err.Error())
			return
		}
		log.Printf("[forwarder] response: %s", b)
		return
	}

	log.Printf(color.HiMagentaString("elapsed time on forward: %v", time.Since(start).Nanoseconds()))
}
