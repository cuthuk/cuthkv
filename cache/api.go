package cache

import (
	"cuthkv/api"
	"cuthkv/config"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"strconv"
)

type RequestItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Ttl   int         `json:"ttl"`
}

var c *Cache

func InitCache() {
	config.InitCfg()
	c = NewCache()
	mux := http.NewServeMux()
	InitApi(mux)
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

func InitApi(mux *http.ServeMux) {
	mux.Handle("/get", api.ResponseWrapper(http.HandlerFunc(GetHandler)))
	mux.Handle("/lget", api.ResponseWrapper(http.HandlerFunc(GetHandler)))
	mux.Handle("/mget", api.ResponseWrapper(http.HandlerFunc(GetHandler)))
	mux.Handle("/set", api.ResponseWrapper(http.HandlerFunc(SetHandler)))
	mux.Handle("/remove", api.ResponseWrapper(http.HandlerFunc(RemoveHandler)))
	mux.Handle("/keys", api.ResponseWrapper(http.HandlerFunc(KeysHandler)))
	mux.Handle("/stat", api.ResponseWrapper(http.HandlerFunc(StatHandler)))
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	innerkey := r.URL.Query().Get("innerkey")
	if len(key) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	item, err := c.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	/*
		это на скорую руку вариант, можно сделать разные типы для итемов
		 и реализовать в них логику
	*/
	switch r.URL.Path {
	case `/lget`:
		ikey, err := strconv.Atoi(innerkey)
		if err != nil {
			http.Error(w, "bad innerkey for list", http.StatusBadRequest)
		}
		if reflect.TypeOf(item).Kind() != reflect.Slice {
			http.Error(w, "value not list", http.StatusBadRequest)
			return
		}
		if ikey > len(item.([]interface{})) {
			http.Error(w, "list key out of range", http.StatusBadRequest)
			return
		}
		item = item.([]interface{})[ikey]
	case `/mget`:
		if reflect.TypeOf(item).Kind() != reflect.Map {
			http.Error(w, "value not dictionary", http.StatusBadRequest)
			return
		}
		item = item.(map[string]interface{})[innerkey]
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"item": item,
	})
}

func SetHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var body []byte
	body, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	item := &RequestItem{}
	err = json.Unmarshal(body, item)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+err.Error(), http.StatusBadRequest)
		return
	}
	_, err = c.Set(item.Key, item.Value, int64(item.Ttl))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+err.Error(), http.StatusForbidden)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"item": item,
	})
}

func RemoveHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if len(key) == 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	result, err := c.Remove(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isDeleted": result,
	})
}

func KeysHandler(w http.ResponseWriter, r *http.Request) {
	pattern := r.URL.Query().Get("pattern")
	keys, err := c.Keys(pattern)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"keys": keys,
	})
}

func StatHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stat": c.Stat(),
	})
}
