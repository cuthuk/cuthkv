package cache

import (
	"cuthkv/config"
	"errors"
	"regexp"
	"sync"
	"time"
)

type (
	OptionsType struct {
		TimeoutGC  int
		DisabledGC bool
	}

	CacheItem struct {
		val interface{}
		ttl int64
	}

	Cache struct {
		store     map[string]CacheItem
		gcTimeout time.Duration
		lock      sync.RWMutex
		cntGcCall uint
		cntHit    uint
		cntMissed uint
	}

	Cacher interface {
		Get(key string) (interface{}, error)
		Set(key string, value interface{}, ttl int64) (interface{}, error)
		Remove(key string) (bool, error)
		Keys(pattern string) ([]string, error)
		Stat() map[string]interface{}
	}

	GCer interface {
		timeoutGC() time.Duration
		GC()
	}
	GCCacher interface {
		GCer
		Cacher
	}
)

func getGcInterval(timeout int) time.Duration {
	return time.Duration(timeout)
}

func NewCache() *Cache {

	c := &Cache{
		store:     make(map[string]CacheItem),
		gcTimeout: getGcInterval(config.Cfg.Options.TimeoutGC),
	}
	if !config.Cfg.Options.DisabledGC {
		go gc(c)
	}
	return c
}

func (c *Cache) timeoutGC() time.Duration {
	return c.gcTimeout * time.Second
}

func (c *Cache) GC() {
	c.lock.Lock()
	c.cntGcCall++
	var key string
	var item CacheItem
	for key, item = range c.store {
		if time.Now().Unix() > item.ttl {
			delete(c.store, key)
		}
	}
	c.lock.Unlock()
}

func gc(c GCCacher) {
	for {
		<-time.After(c.timeoutGC())
		c.GC()
	}
}

func (c *Cache) Stat() map[string]interface{} {
	c.lock.RLock()
	stat := map[string]interface{}{
		"hit":    c.cntHit,
		"missed": c.cntMissed,
		"gcCall": c.cntGcCall,
	}
	c.lock.RUnlock()
	stat["len"] = c.Len()
	return stat
}

func (c *Cache) find(key string) (item CacheItem, found bool) {
	c.cntHit++
	c.lock.RLock()
	item, found = c.store[key]
	c.lock.RUnlock()
	if !found {
		c.cntMissed++
	}

	return
}

func (c *Cache) Len() int {
	c.lock.RLock()
	length := len(c.store)
	c.lock.RUnlock()
	return length
}

// получение ключа из хранилища
func (c *Cache) Get(key string) (interface{}, error) {
	item, ok := c.find(key)
	if !ok {
		return nil, errors.New("item not found")
	}
	return item.val, nil
}

// запись в хранилище
func (c *Cache) Set(key string, value interface{}, ttl int64) (interface{}, error) {
	ttl = time.Now().Unix() + ttl
	c.lock.Lock()
	c.store[key] = CacheItem{
		val: value,
		ttl: ttl,
	}
	c.lock.Unlock()
	return value, nil
}

// удаление ключа из хранилища
func (c *Cache) Remove(key string) (bool, error) {
	_, ok := c.find(key)
	if !ok {
		return false, errors.New("item not found")
	}
	c.lock.Lock()
	delete(c.store, key)
	c.lock.Unlock()
	return true, nil
}

// вытаскивает ключи из хранилища и фильтрует по переданному шаблону
func (c *Cache) Keys(pattern string) ([]string, error) {
	storeLen := c.Len()
	if storeLen == 0 {
		return nil, errors.New("store empty")
	}
	keys := make([]string, 0, storeLen)
	c.lock.RLock()
	for key := range c.store {
		keys = append(keys, key)
	}
	c.lock.RUnlock()
	return filterKeys(keys, pattern)
}

// фильтрует ключи в мапе по шаблону
func filterKeys(inkeys []string, pattern string) ([]string, error) {
	var keys []string
	// без шаблона возвращаем все полученные ключи
	if len(pattern) == 0 {
		return inkeys, nil
	}
	pattern = "^" + pattern + "$"
	in := make(chan string, config.Cfg.Options.FilterKeysWorkers)
	out := make(chan string)
	finished := make(chan bool)
	wg := &sync.WaitGroup{}
	compiledRegexp, _ := regexp.Compile(pattern)
	for i := 0; i < config.Cfg.Options.FilterKeysWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for inkey := range in {
				matched := compiledRegexp.MatchString(inkey)
				if matched {
					out <- inkey
				}
			}
		}()
	}
	go func() {
		for key := range out {
			keys = append(keys, key)
		}
		finished <- true
	}()

	for _, inkey := range inkeys {
		in <- inkey
	}
	close(in)
	wg.Wait()
	close(out)
	<-finished
	return keys, nil
}
