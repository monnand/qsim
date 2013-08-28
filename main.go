package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
)

var argvHTTPPort = flag.Int("port", 8080, "HTTP service port number")
var argvNrServers = flag.Int("n", 1, "number of servers")

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU() + 2)
	r := NewRecursiveDelay(*argvNrServers)
	addr := fmt.Sprintf("0.0.0.0:%v", *argvHTTPPort)
	log.Fatal(http.ListenAndServe(addr, r))
}
