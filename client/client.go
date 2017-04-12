package client

import (
	"bytes"
	"cuthkv/api"
	"cuthkv/config"
	"encoding/json"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type RequestItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Ttl   int         `json:"ttl"`
}

type StatItemType struct {
	Store string      `json:"store"`
	Stat  interface{} `json:"stat"`
}

var StoreStat map[string]interface{}

func InitClient() {
	config.InitCfg()
	storeList := config.Cfg.Cluster.StoreList
	if len(storeList) == 0 {
		log.Fatalln("storeList empty!!!")
	}
	StoreStat = make(map[string]interface{}, len(storeList))
	go statCrawler()
	targets := make([]*url.URL, 0, len(storeList))
	for _, item := range storeList {
		targets = append(targets, &url.URL{
			Scheme: "http",
			Host:   item,
		})
	}
	proxy := newReverseProxy(targets)
	mux := http.NewServeMux()
	mux.Handle("/stat", api.ResponseWrapper(http.HandlerFunc(statHandler)))
	mux.Handle("/keys", api.ResponseWrapper(http.HandlerFunc(keysHandler)))
	mux.Handle("/", proxy)
	err := http.ListenAndServe(config.Cfg.Server.Host+":"+config.Cfg.Server.Port, mux)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered after %s \n", r)
		}
	}()
}

func calcKeyHash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

func getStoreServer(key string) int {
	hash := calcKeyHash(key)
	return int(hash % uint32(len(config.Cfg.Cluster.StoreList)))
}

func newReverseProxy(targets []*url.URL) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		target := targets[getKey(req)]
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}
	return &httputil.ReverseProxy{Director: director}
}

func getKey(r *http.Request) int {
	item := &RequestItem{}
	switch r.Method {
	case http.MethodPost:
		item.Key = r.URL.Query().Get("key")
		var err error
		var body []byte
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			return 0
		}
		rdr2 := ioutil.NopCloser(bytes.NewBuffer(body))
		r.Body = rdr2
		err = json.Unmarshal(body, item)
		if err != nil {
			return 0
		}
	default:
		item.Key = r.URL.Query().Get("key")
	}
	return getStoreServer(item.Key)
}

func statHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stat": StoreStat,
	})
}

func keysHandler(w http.ResponseWriter, r *http.Request) {
	wg := &sync.WaitGroup{}
	keysCh := make(chan []string)
	keys := []string{}
	done := make(chan bool)
	for _, store := range config.Cfg.Cluster.StoreList {
		wg.Add(1)
		go func() {
			storeKeys := make(map[string][]string)
			defer wg.Done()
			req, _ := http.NewRequest("GET", "http://"+store+"/keys", nil)
			req.URL.RawQuery = r.URL.RawQuery
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("host %s key request failed: %s", store, err.Error())
				return
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)

			json.Unmarshal(body, storeKeys)
			keysCh <- storeKeys["keys"]
		}()
	}
	go func() {
		for storekeys := range keysCh {
			keys = append(keys, storekeys...)
		}
		done <- true
	}()
	wg.Wait()
	close(keysCh)
	<-done
	json.NewEncoder(w).Encode(map[string]interface{}{
		"keys": keys,
	})
}

func statCrawler() {
	<-time.After(time.Duration(config.Cfg.Cluster.StatCrawlerTimeout) * time.Second)
	wg := &sync.WaitGroup{}
	statCh := make(chan *StatItemType)
	done := make(chan bool)
	for _, store := range config.Cfg.Cluster.StoreList {
		wg.Add(1)
		go func() {
			statItem := &StatItemType{}
			defer wg.Done()
			defer func() {
				statCh <- statItem
			}()
			resp, err := http.Get("http://" + store + "/stat")
			if err != nil {
				statItem.Store = store
				statItem.Stat = err.Error()
				return
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, statItem)
			statItem.Store = store
		}()
	}
	go func() {
		for statItem := range statCh {
			StoreStat[statItem.Store] = statItem.Stat
		}
		done <- true
	}()
	wg.Wait()
	close(statCh)
	<-done
}
