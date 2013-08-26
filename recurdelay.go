package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	reqChan chan *request
}

func NewRecursiveDelay(nrServers int) *RecursiveDelay {
	ret := new(RecursiveDelay)
	ret.reqChan = make(chan *request)
	if nrServers <= 0 {
		nrServers = 1
	}
	for i := 0; i < nrServers; i++ {
		go ret.serveRequest()
	}
	return ret
}

func (self *RecursiveDelay) serveRequest() {
	for req := range self.reqChan {
		res := new(response)
		res.start = time.Now()
		req.path.Delay()
		res.end = time.Now()
		res.err = nil
		req.respChan <- res
	}
}

func (self *RecursiveDelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

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

	fmt.Fprintf(w, "site: %v; queuing delay: %v; service time: %v\n", path.Sites[0].Name(), res.start.Sub(start), res.end.Sub(res.start))

	body, err := path.Forward()
	if err != nil {
		fmt.Fprintf(w, "Error occored on site %v when forwarding: %v\n", path.Sites[0].Name(), err)
		return
	}
	w.Write(body)
	return
}
