package main

import (
	"errors"
	"log"
	"net/rpc"

	kv "github.com/UC-DBT-Group9/CAPP/internal"
)

// Configuration for a storage worker. Used by the KVServer.
type KVStorageWorkerConfig struct {
	RPCServerPort string
}

// Storage worker metadata.
type KVStorageWorker struct {
	RPCClient     *rpc.Client // RPC client to communicate with the server.
	RPCServerPort string      // Port the worker is listening on for RPC.
	MinKey        string      // The smallest key the worker has in storage.
	MaxKey        string      // the largets key the worker has in storage.
}

// The storage coordinator/kv server.
type KVServer struct {
	StorageWorkers []KVStorageWorker // List of all current storage workers.
}

// KVServer constructor. Takes in a list of KVStorageWorkerConfigs for connecting to a storage cluster.
func NewKVServer(storageWorkerConfigs []KVStorageWorkerConfig) (*KVServer, error) {
	server := &KVServer{}
	server.StorageWorkers = make([]KVStorageWorker, len(storageWorkerConfigs))
	err := server.connectToStorageWorkers(storageWorkerConfigs)
	if err != nil {
		return nil, err
	}
	return server, nil
}

// Initialize the RPC connections to the given list of storage worker configurations.
func (server *KVServer) connectToStorageWorkers(storageWorkerConfigs []KVStorageWorkerConfig) error {
	for i := range storageWorkerConfigs {
		server.StorageWorkers[i].RPCServerPort = storageWorkerConfigs[i].RPCServerPort
		client, err := rpc.DialHTTP("tcp", server.StorageWorkers[i].RPCServerPort)
		if err != nil {
			log.Println("dialing:", err)
		}
		server.StorageWorkers[i].RPCClient = client
	}
	return nil
}

// Store a key-value pair in the storage cluster.
func (server *KVServer) Put(key string, value kv.Value) error {
	put_request := &kv.PutArgs{Key: key, Value: value}
	var put_response kv.PutReply

	log.Println("Calling Put...")
	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		if worker.RPCClient != nil {
			err := worker.RPCClient.Call("KVStore.Put", put_request, &put_response)
			if err != nil {
				log.Println("put error:", err)
			}
			log.Println("Put success:", put_response)

			if worker.MinKey == "" || worker.MinKey > key {
				worker.MinKey = key
			}
			if worker.MaxKey == "" || worker.MaxKey < key {
				worker.MaxKey = key
			}
		}
	}
	return nil
}

// Retreive the  value associated with `key` in the cluster.
func (server *KVServer) Get(key string) (kv.Value, error) {
	get_request := &kv.GetArgs{Key: key}
	var get_response = &kv.GetReply{}

	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		if key >= worker.MinKey && key <= worker.MaxKey {
			log.Println("Calling get for key:", key)
			err := worker.RPCClient.Call("KVStore.Get", get_request, get_response)
			if err != nil {
				return kv.NullValue(), err
			}
			log.Println("Got key:", get_response.Value)
			return get_response.Value, nil
		}
	}
	return kv.NullValue(), errors.New("not found.")
}

func (server *KVServer) Select(predicate *kv.Expr) ([]kv.Value, error) {
	req := &kv.SelectArgs{Predicate: predicate}
	var resp = &kv.SelectReply{}
	var values []kv.Value

	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		log.Println("Calling select with pred:", predicate)
		err := worker.RPCClient.Call("KVStore.Select", req, resp)
		if err != nil {
			return nil, err
		}
		if resp.Values != nil {
			return resp.Values, nil
		}
	}

	return values, nil
}

func main() {
	// Demo of using the API.
	log.Println("Dialing the server...")
	storageWorkerConfigs := []KVStorageWorkerConfig{{RPCServerPort: ":6789"}, {RPCServerPort: ":6790"}}
	server, err := NewKVServer(storageWorkerConfigs)
	if err != nil {
		log.Fatal("connect error:", err)
	}

	err = server.Put("Hello", kv.StringValue("World"))
	if err != nil {
		log.Fatal("put error: ", err)
	}
	err = server.Put("num1", kv.IntValue(1))
	if err != nil {
		log.Fatal("put error: ", err)
	}
	err = server.Put("num2", kv.IntValue(2))
	if err != nil {
		log.Fatal("put error: ", err)
	}

	val, err := server.Get("Hello")
	if err != nil {
		log.Fatal("get error: ", err)
	}
	val, err = server.Get("hello")
	if err != nil {
		log.Println("get error:", err)
	}
	log.Println("Got:", val)

	pred := kv.Binary(kv.OpLT, kv.Root(), kv.Literal(kv.IntValue(2)))
	vals, err := server.Select(&pred)
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range vals {
		log.Println(v.Int)
	}
}
