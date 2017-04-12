package main

import (
	"bytes"
	"cuthkv/cache"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type countersType struct {
	CntSet    uint32
	FailedSet uint32
	CntGet    uint32
	FailedGet uint32
}

const Url = "http://localhost:3090"

var Counters countersType

func main() {
	Counters = countersType{
		CntSet:    0,
		FailedSet: 0,
		CntGet:    0,
		FailedGet: 0,
	}
	TestTtl := 30
	numbPtr := flag.Int("numb", 1, "an int")
	checkGcPtr := flag.Bool("chkgc", false, "a bool")
	flag.Parse()
	wg := &sync.WaitGroup{}
	for i := 0; i < *numbPtr; i++ {
		wg.Add(1)
		go func(ikey int) {
			defer wg.Done()
			time.Sleep(time.Duration(5) * time.Millisecond)
			key := strconv.Itoa(ikey)
			testSet(cache.RequestItem{
				Key:   key,
				Value: "string",
				Ttl:   TestTtl,
			})
			testGet(key)
			if *checkGcPtr {
				time.Sleep(time.Duration(TestTtl) * time.Second)
				testGet(key)
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("%#v", Counters)
}

func testGetSet() {

}

func testSet(item cache.RequestItem) {
	atomic.AddUint32(&Counters.CntSet, 1)
	json, _ := json.Marshal(item)
	req, _ := http.NewRequest("POST", Url+"/set", bytes.NewBuffer(json))
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK{
		atomic.AddUint32(&Counters.FailedSet, 1)
		return
	}
	resp.Body.Close()
}

func testGet(key string) {
	atomic.AddUint32(&Counters.CntGet, 1)
	req, _ := http.NewRequest("GET", Url+"/get?key="+key, nil)
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		atomic.AddUint32(&Counters.FailedGet, 1)
		return
	}
	resp.Body.Close()
}

func testRemove() {

}
