package main

import (
	"bytes"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"

	kv "github.com/UC-DBT-Group9/CAPP/internal"
)

func main() {
	port := bytes.NewBufferString(":")
	args := os.Args[1:]
	if len(args) == 0 {
		port.WriteString("6789")
	} else if len(args) >= 2 && args[0] == "--port" {
		port.WriteString(args[1])
	}

	kvserver := kv.NewKVStore()
	rpc.Register(kvserver)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", port.String())
	if err != nil {
		log.Fatal("listen error:", err)
	}
	log.Println("Listening on port", port.String())
	http.Serve(l, nil)
}
