package kv

import (
	"log"
	"sync"
)

type SelectArgs struct {
	Predicate *Expr
}
type SelectReply struct {
	Values []Value
}

type PutArgs struct {
	Key   string
	Value Value
}
type PutReply bool

type GetArgs struct {
	Key string
}

type GetReply struct {
	Value Value
}

type KVStore struct {
	storage map[string]Value
	mu      sync.Mutex
}

func NewKVStore() *KVStore {
	return &KVStore{storage: make(map[string]Value)}
}

func (store *KVStore) Put(args *PutArgs, reply *PutReply) error {
	log.Println("Received put request:", args)
	if args.Key == "" {
		*reply = false
		return nil
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	store.storage[args.Key] = args.Value
	*reply = true

	return nil
}

func (store *KVStore) Get(args *GetArgs, reply *GetReply) error {
	log.Println("Received get request:", args)

	store.mu.Lock()
	defer store.mu.Unlock()

	reply.Value = store.storage[args.Key]
	return nil
}

func (store *KVStore) Select(args *SelectArgs, reply *SelectReply) error {
	var values []Value
	if args.Predicate == nil {
		for _, v := range store.storage {
			values = append(values, v)
		}
	} else {
		for _, v := range store.storage {
			res, err := args.Predicate.Eval(v)
			if err != nil {
				log.Println(err)
			}
			if res.Kind == KindBool && res.Bool {
				values = append(values, v)
			}
		}
	}
	reply.Values = values
	return nil
}
