package main

import (
	"log"
	"time"

	kv "github.com/UC-DBT-Group9/CAPP/internal"
	phys "github.com/UC-DBT-Group9/CAPP/internal/physical"
)

func main() {
	// Demo of using the API.
	log.Println("Dialing the server...")
	storageWorkerConfigs := []kv.KVStorageWorkerConfig{{RPCServerPort: ":6789"}, {RPCServerPort: ":6790"}}
	server, err := kv.NewKVServer(storageWorkerConfigs)
	if err != nil {
		log.Fatal("connect error:", err)
	}

	obj := kv.CSVObj{
		Data: []byte("id,name,age\n1,Mason,30\n2,Shiv,25\n3,Abhi,25\n4,test,2\n5,test1,3"),
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
	err = server.Put("people1.csv", obj)
	if err != nil {
		log.Fatal("put error: ", err)
	}

	pred := kv.Binary(kv.OpGT, kv.AttrNum(2), kv.Literal(kv.IntValue(24)))

	start := time.Now()
	rs := phys.NewRemoteScan(&pred, []kv.NamedExpr{}, "people.csv", server)
	_, tuples, err := rs.NextBatch()
	end := time.Now()
	elapsed := end.Sub(start)
	log.Println("Remote scan:", elapsed)
	for _, t := range tuples {
		log.Println(t)
	}

	start = time.Now()
	ls := phys.NewLocalScan(&pred, []kv.NamedExpr{}, "people1.csv", server)
	_, tuples, err = ls.NextBatch()
	end = time.Now()
	elapsed = end.Sub(start)
	log.Println("Local scan:", elapsed)
	for _, t := range tuples {
		log.Println(t)
	}

	start = time.Now()
	lim := phys.NewLimitNode(10, ls)
	_, tuples, err = lim.NextBatch()
	end = time.Now()
	elapsed = end.Sub(start)
	log.Println("Limit:", elapsed)
	log.Println(len(tuples))
}
