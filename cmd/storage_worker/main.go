package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"time"

	kv "github.com/UC-DBT-Group9/CAPP/internal"
)

func main() {
	port := flag.String("port", "6789", "port to listen on")
	netMBps := flag.Float64("net-mb-per-sec", 0, "throttle: network bandwidth back to the coordinator in MB/s (0 = unlimited)")
	scanMBps := flag.Float64("scan-mb-per-sec", 0, "throttle: storage scan speed for Select in MB/s (0 = unlimited)")
	slots := flag.Int("select-slots", 0, "throttle: max Selects running at once (0 = unlimited)")
	latencyMs := flag.Int("latency-ms", 0, "throttle: fixed delay per request in milliseconds")
	flag.Parse()

	const mb = 1 << 20
	throttle := kv.Throttle{
		BandwidthBytesPerSec: *netMBps * mb,
		ScanBytesPerSec:      *scanMBps * mb,
		SelectSlots:          *slots,
		RequestLatency:       time.Duration(*latencyMs) * time.Millisecond,
	}

	kvserver := kv.NewKVStore()
	kvserver.SetThrottle(throttle)
	rpc.Register(kvserver)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	log.Printf("Listening on port :%s (throttle: %+v)", *port, throttle)
	http.Serve(l, nil)
}
