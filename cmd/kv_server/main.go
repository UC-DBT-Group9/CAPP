package main

import (
	"log"
	"time"

	kv "github.com/UC-DBT-Group9/CAPP/internal"
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

	pred := kv.Binary(kv.OpGT, kv.Field("age"), kv.Literal(kv.IntValue(24)))

	start := time.Now()
	rs := &kv.Filter{Predicate: pred, Input: kv.NewRemoteScan("people.csv", server)}
	_, tuples, err := rs.Execute()
	if err != nil {
		log.Fatal("remote scan error: ", err)
	}
	end := time.Now()
	elapsed := end.Sub(start)
	log.Println("Remote scan:", elapsed)
	for _, t := range tuples {
		log.Println(t)
	}

	start = time.Now()
	ls := &kv.Filter{Predicate: pred, Input: kv.NewLocalScan("people1.csv", server)}
	_, tuples, err = ls.Execute()
	if err != nil {
		log.Fatal("local scan error: ", err)
	}
	end = time.Now()
	elapsed = end.Sub(start)
	log.Println("Local scan:", elapsed)
	for _, t := range tuples {
		log.Println(t)
	}

	start = time.Now()
	planner := kv.NewPlanner(server)
	plan, err := planner.Plan(kv.LimitNode{
		NumRows: 2,
		Input: kv.ProjectNode{
			Attributes: []string{"id", "name"},
			Input: kv.FilterNode{
				Predicate: pred,
				Input:     kv.TableScanNode{Source: "people.csv"},
			},
		},
	})
	if err != nil {
		log.Fatal("plan error: ", err)
	}
	_, tuples, err = plan.Execute()
	if err != nil {
		log.Fatal("query error: ", err)
	}
	end = time.Now()
	elapsed = end.Sub(start)
	log.Println("Planned query:", elapsed)
	log.Println(len(tuples))
	usage, err := server.GetUsage()
	cost := usage.Cost(kv.DefaultRates())
	log.Println(cost)
	log.Printf("%.6f", cost.Total)
}
