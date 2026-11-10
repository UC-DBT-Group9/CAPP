package kv

import (
	"errors"
	"log"
	"net/rpc"
	"time"
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
	computeStarted time.Time
	now            func() time.Time  // Allows for substituting clock functions.
	StorageWorkers []KVStorageWorker // List of all current storage workers.
}

// KVServer constructor. Takes in a list of KVStorageWorkerConfigs for connecting to a storage cluster.
func NewKVServer(storageWorkerConfigs []KVStorageWorkerConfig) (*KVServer, error) {
	server := &KVServer{computeStarted: time.Now(), now: time.Now}
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
func (server *KVServer) Put(key string, obj CSVObj) error {
	put_request := &PutArgs{Key: key, CSVObj: obj}
	var put_response PutReply
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
func (server *KVServer) Get(key string) (CSVObj, error) {
	get_request := &GetArgs{Key: key}
	var get_response = &GetReply{}

	for i := range server.StorageWorkers {
		worker := &server.StorageWorkers[i]
		if worker.RPCClient != nil && key >= worker.MinKey && key <= worker.MaxKey {
			log.Println("Calling get for key:", key)
			err := worker.RPCClient.Call("KVStore.Get", get_request, get_response)
			if err != nil {
				return CSVObj{}, err
			}
			return get_response.CSVObj, nil
		}
	}
	return CSVObj{}, errors.New("not found")
}

func (server *KVServer) Select(key string, predicate *Expr, project []string) (*SelectReply, error) {
	req := &SelectArgs{Key: key, Predicate: predicate, Project: project}
	var resp = &SelectReply{}

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

func (server *KVServer) Usage() CoordinatorUsage {
	if server.computeStarted.IsZero() {
		return CoordinatorUsage{}
	}
	clock := server.now
	if clock == nil {
		clock = time.Now
	}
	now := clock()

	elapsed := max(now.Sub(server.computeStarted), 0)
	return CoordinatorUsage{ComputeTime: elapsed}
}
