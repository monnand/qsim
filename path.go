package main

import (
	"bytes"
	"code.google.com/p/probab/dst"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Distribution struct {
	Name       string             `json:"type"`
	Parameters map[string]float64 `json:"parameters,omitempty"`
}

type SiteSpec struct {
	Addr  string        `json:"addr"`
	Stime string        `json:"service-time,omitempty"`
	Sdist *Distribution `json:"service-time-distribution,omitempty"`
}

func (self *SiteSpec) Name() string {
	return self.Addr
}

func sleepAndDelay(duration string) {
	if len(duration) > 0 {
		d, e := time.ParseDuration(duration)
		if e == nil {
			time.Sleep(d)
		}
	}
}

func (self *Distribution) nextRandomNumber() float64 {
	switch strings.ToLower(self.Name) {
	case "poisson":
		lambda := 500.0
		if l, ok := self.Parameters["lambda"]; ok {
			lambda = l
		}
		return dst.ExponentialNext(lambda)
	}
	return 0
}

func (self *Distribution) Sleep() {
	if self == nil {
		return
	}
	d, _ := time.ParseDuration(fmt.Sprint("%vs", self.nextRandomNumber()))
	time.Sleep(d)
}

func (self *SiteSpec) DelayBySleeping() {
	sleepAndDelay(self.Stime)
	self.Sdist.Sleep()
}

type PathSpec struct {
	Sites  []*SiteSpec `json:"sites"`
	PathId string      `json:"id"`
}

func (self *PathSpec) forwardFromFirst() (resp []byte, err error) {
	if len(self.Sites) == 0 {
		resp = []byte(fmt.Sprintf("finished path %v\n", self.PathId))
		err = nil
		return
	}
	addr := self.Sites[0].Addr
	jbytes, err := json.Marshal(self)
	if err != nil {
		return
	}
	bodyReader := bytes.NewBuffer(jbytes)
	res, err := http.Post(addr, "text/json", bodyReader)
	if err != nil {
		return
	}
	defer res.Body.Close()

	resp, err = ioutil.ReadAll(res.Body)
	return
}

func (self *PathSpec) String() string {
	buf := new(bytes.Buffer)
	buf.WriteString(fmt.Sprintf("id: %v\n", self.PathId))

	for _, site := range self.Sites {
		buf.WriteString(fmt.Sprintf("%v\n", site))
	}
	return buf.String()
}

func (self *PathSpec) Delay() error {
	site := self.Sites[0]
	site.DelayBySleeping()
	return nil
}

func (self *PathSpec) Forward() ([]byte, error) {
	path := new(PathSpec)
	path.Sites = self.Sites[1:]
	path.PathId = self.PathId
	return path.forwardFromFirst()
}
