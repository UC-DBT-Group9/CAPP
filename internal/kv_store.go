package kv

import (
	"fmt"
	"io"
	"log"
	"sync"
)

type SelectArgs struct {
	Key       string
	Predicate *Expr
	Project   []string
}
type SelectReply struct {
	Schema Schema
	Tuples []Tuple
}

type PutArgs struct {
	Key    string
	CSVObj CSVObj
}
type PutReply bool

type GetArgs struct {
	Key string
}

type GetReply struct {
	CSVObj CSVObj
}

type KVStore struct {
	storage map[string]CSVObj
	mu      sync.Mutex
}

func NewKVStore() *KVStore {
	return &KVStore{storage: make(map[string]CSVObj)}
}

func (store *KVStore) Put(args *PutArgs, reply *PutReply) error {
	log.Println("Received put request:", args.Key)
	*reply = false
	if args.Key == "" {
		*reply = false
		return nil
	}
	if _, err := args.CSVObj.Parse(); err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	store.storage[args.Key] = args.CSVObj
	*reply = true

	return nil
}

func (store *KVStore) Get(args *GetArgs, reply *GetReply) error {
	log.Println("Received get request:", args)

	store.mu.Lock()
	defer store.mu.Unlock()

	obj, found := store.storage[args.Key]
	if !found {
		return fmt.Errorf("key %s not found", args.Key)
	}
	reply.CSVObj = obj
	return nil
}

func (store *KVStore) Select(args *SelectArgs, reply *SelectReply) error {
	store.mu.Lock()
	csvObj, found := store.storage[args.Key]
	store.mu.Unlock()
	if !found {
		return fmt.Errorf("key %s not found", args.Key)
	}

	predicate, err := ResolvePredicate(csvObj.Schema, args.Predicate)
	if err != nil {
		return err
	}
	positions, schema, err := ResolveProjection(csvObj.Schema, args.Project)
	if err != nil {
		return err
	}
	result := SelectReply{Schema: schema}

	iter := csvObj.Iterator()

	for {
		tuple, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		match, err := MatchesPredicate(*tuple, predicate)
		if err != nil {
			return err
		}
		if !match {
			continue
		}
		result.Tuples = append(result.Tuples, ProjectTuple(*tuple, positions))
	}
	*reply = result
	return nil
}
