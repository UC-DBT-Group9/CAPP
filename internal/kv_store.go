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

	result := SelectReply{Schema: Schema{Name: csvObj.Schema.Name}}
	var attrPositions []int

	if len(args.Project) == 0 {
		for i, attr := range csvObj.Schema.Attributes {
			attrPositions = append(attrPositions, i)
			result.Schema.Attributes = append(result.Schema.Attributes, attr)
		}
	} else {
		for _, name := range args.Project {
			position := -1
			for i, attr := range csvObj.Schema.Attributes {
				if attr.Name == name {
					position = i
					break
				}
			}
			if position == -1 {
				return fmt.Errorf("unknown projection column %q", name)
			}
			attrPositions = append(attrPositions, position)
			result.Schema.Attributes = append(result.Schema.Attributes, csvObj.Schema.Attributes[position])
		}
	}

	iter := csvObj.Iterator()

	for {
		tuple, err := iter.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if args.Predicate != nil {
			fields := make(map[string]Value, len(tuple.Values))
			for i, attr := range csvObj.Schema.Attributes {
				fields[attr.Name] = tuple.Values[i]
			}
			res, err := args.Predicate.Eval(ObjectValue(fields))
			if err != nil {
				return err
			}
			if res.DataType != DataTypeBool {
				return fmt.Errorf("predicate must return bool, got %s", res.DataType)
			}
			if !res.Bool {
				continue
			}
		}
		projected := Tuple{Values: make([]Value, len(attrPositions))}
		for i, position := range attrPositions {
			projected.Values[i] = tuple.Values[position]
		}
		result.Tuples = append(result.Tuples, projected)
	}
	*reply = result
	return nil
}
