package kv

import (
	"log"
	"sync"
)

type PutArgs struct {
	Key   string
	Value []byte
}
type PutReply bool

type GetArgs struct {
	Key string
}

type GetReply struct {
	Value []byte
}

type KVStore struct {
	storage map[string][]byte
	mu      sync.Mutex
}

func NewKVStore() *KVStore {
	return &KVStore{storage: make(map[string][]byte)}
}

func (server *KVStore) Put(args *PutArgs, reply *PutReply) error {
	log.Println("Received put request:", args)
	if args.Key == "" {
		*reply = false
		return nil
	}

	server.mu.Lock()
	defer server.mu.Unlock()

	server.storage[args.Key] = args.Value
	*reply = true

	return nil
}

func (server *KVStore) Get(args *GetArgs, reply *GetReply) error {
	log.Println("Received get request:", args)

	server.mu.Lock()
	defer server.mu.Unlock()

	reply.Value = server.storage[args.Key]
	return nil
}
