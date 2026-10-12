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

// Store a CSV object in the storage cluster.
func (server *KVServer) Put(key string, obj kv.CSVObj) error {
	put_request := &kv.PutArgs{Key: key, CSVObj: obj}
	var put_response kv.PutReply
	stored := false

	log.Println("Calling Put...")
	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		if worker.RPCClient != nil {
			err := worker.RPCClient.Call("KVStore.Put", put_request, &put_response)
			if err != nil {
				return err
			}
			if !put_response {
				return errors.New("put rejected")
			}
			stored = true
			log.Println("Put success:", put_response)

			if worker.MinKey == "" || worker.MinKey > key {
				worker.MinKey = key
			}
			if worker.MaxKey == "" || worker.MaxKey < key {
				worker.MaxKey = key
			}
		}
	}
	if !stored {
		return errors.New("no connected storage workers")
	}
	return nil
}

// Retrieve the CSV object associated with key in the cluster.
func (server *KVServer) Get(key string) (kv.CSVObj, error) {
	get_request := &kv.GetArgs{Key: key}
	var get_response = &kv.GetReply{}

	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		if worker.RPCClient != nil && key >= worker.MinKey && key <= worker.MaxKey {
			log.Println("Calling get for key:", key)
			err := worker.RPCClient.Call("KVStore.Get", get_request, get_response)
			if err != nil {
				return kv.CSVObj{}, err
			}
			return get_response.CSVObj, nil
		}
	}
	return kv.CSVObj{}, errors.New("not found")
}

func (server *KVServer) Select(key string, predicate *kv.Expr, project []string) (*kv.SelectReply, error) {
	req := &kv.SelectArgs{Key: key, Predicate: predicate, Project: project}
	var resp = &kv.SelectReply{}

	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		if worker.RPCClient == nil || key < worker.MinKey || key > worker.MaxKey {
			continue
		}
		log.Println("Calling select with pred:", predicate)
		err := worker.RPCClient.Call("KVStore.Select", req, resp)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	return nil, errors.New("not found")
}

func main() {
	// Demo of using the API.
	log.Println("Dialing the server...")
	storageWorkerConfigs := []KVStorageWorkerConfig{{RPCServerPort: ":6789"}, {RPCServerPort: ":6790"}}
	server, err := NewKVServer(storageWorkerConfigs)
	if err != nil {
		log.Fatal("connect error:", err)
	}

	obj := kv.CSVObj{
		Data: []byte("id,name,age\n1,Mason,30\n2,Shiv,25\n"),
		Schema: kv.Schema{Name: "people", Attributes: []kv.Attribute{
			{Name: "id", DataType: kv.DataTypeInt, Required: true},
			{Name: "name", DataType: kv.DataTypeString, Required: true},
			{Name: "age", DataType: kv.DataTypeInt, Required: true},
		}},
		Opts: kv.CSVOpts{HasHeader: true},
	}
	err = server.Put("people.csv", obj)
	if err != nil {
		log.Fatal("put error: ", err)
	}

	stored, err := server.Get("people.csv")
	if err != nil {
		log.Fatal("get error: ", err)
	}
	log.Println("Got CSV:", string(stored.Data))

	pred := kv.Binary(kv.OpGT, kv.Field("age"), kv.Literal(kv.IntValue(25)))
	result, err := server.Select("people.csv", &pred, []string{"name"})
	if err != nil {
		log.Fatal(err)
	}
	for _, tuple := range result.Tuples {
		log.Println(tuple.Values[0].String)
	}
}
