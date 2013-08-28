package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"time"
)

type response struct {
	err   error
	start time.Time
	end   time.Time
}

type request struct {
	respChan chan<- *response
	path     *PathSpec
}

type RecursiveDelay struct {
	reqChan   chan *request
	stopChan  chan bool
	autoScale bool
	nrServers int32
}

func NewRecursiveDelay(nrServers int) *RecursiveDelay {
	ret := new(RecursiveDelay)
	ret.reqChan = make(chan *request)
	ret.stopChan = make(chan bool)
	if nrServers <= 0 {
		nrServers = 1
		ret.autoScale = true
	}

	// We should always make sure there is
	// at least one server.
	go ret.serveRequest(nil)
	for i := 0; i < nrServers-1; i++ {
		go ret.serveRequest(ret.stopChan)
	}
	return ret
}

func (self *RecursiveDelay) serveRequest(stopChan <-chan bool) {
	atomic.AddInt32(&self.nrServers, int32(1))
	defer atomic.AddInt32(&self.nrServers, int32(-1))
	for {
		select {
		case req := <-self.reqChan:
			if req == nil {
				return
			}
			res := new(response)
			res.start = time.Now()
			req.path.Delay()
			res.end = time.Now()
			res.err = nil
			req.respChan <- res
		case <-stopChan:
			return
		}
	}
}

func (self *RecursiveDelay) scale(queuingDelay, serviceTime time.Duration) {
	if queuingDelay*2 < serviceTime {
		// Too small queuing delay, we are wasting resources.
		self.scaleDown()
	}
	if queuingDelay > serviceTime*2 {
		// No enough resources.
		self.scaleUp()
	}
}

func (self *RecursiveDelay) scaleUp() {
	go self.serveRequest(self.stopChan)
}

func (self *RecursiveDelay) scaleDown() {
	select {
	case self.stopChan <- true:
	default:
		return
	}
}

func (self *RecursiveDelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.URL.Path == "/nrservers" {
		fmt.Fprint(w, "%v\n", atomic.LoadInt32(&self.nrServers))
		return
	}

	pathBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(w, "Error occored on reading body: %v\n", err)
		return
	}

	path := new(PathSpec)
	err = json.Unmarshal(pathBody, path)
	if err != nil {
		fmt.Fprintf(w, "Error occored on json decoding: %v\n", err)
		return
	}
	if len(path.Sites) == 0 {
		fmt.Fprintf(w, "You should at least specify one site\n")
		return
	}

	ch := make(chan *response)
	req := new(request)
	req.path = path
	req.respChan = ch
	start := time.Now()

	self.reqChan <- req
	res := <-ch

	if res.err != nil {
		fmt.Fprintf(w, "Error occored on site %v: %v\n", path.Sites[0].Name(), err)
		return
	}

	qdelay := res.start.Sub(start)
	stime := res.end.Sub(res.start)
	fmt.Fprintf(w, "site: %v; queuing delay: %v; service time: %v\n", path.Sites[0].Name(), qdelay, stime)
	if self.autoScale {
		self.scale(qdelay, stime)
	}

	body, err := path.Forward()
	if err != nil {
		fmt.Fprintf(w, "Error occored on site %v when forwarding: %v\n", path.Sites[0].Name(), err)
		return
	}
	w.Write(body)
	return
}
