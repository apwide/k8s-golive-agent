package cache

import (
	"context"
	"encoding/json"
	cache2 "github.com/patrickmn/go-cache"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type GoliveCache struct {
	cache  *cache2.Cache
	mu     sync.Mutex
	ctx    context.Context
	logger klog.Logger
}

func NewCache(expiration time.Duration) *GoliveCache {
	return &GoliveCache{
		cache:  cache2.New(expiration, expiration),
		logger: klog.FromContext(context.Background()),
	}
}

func (c *GoliveCache) get(key interface{}) (interface{}, bool) {
	jsonKey, ok := c.toJson(key, "key")
	if ok {
		return c.cache.Get(jsonKey)
	} else {
		return nil, ok
	}
}

func (c *GoliveCache) set(key interface{}, item interface{}) {
	jsonKey, ok := c.toJson(key, "key")
	if !ok {
		return
	}
	payload, ok := c.toJson(item, "value")
	if !ok {
		return
	}
	c.cache.Set(jsonKey, payload, cache2.DefaultExpiration)
}

func (c *GoliveCache) Delete(key interface{}) {
	jsonKey, ok := c.toJson(key, "key")
	if !ok {
		return
	}
	c.cache.Delete(jsonKey)
}

func (c *GoliveCache) isUpdateToDate(key interface{}, info interface{}) bool {
	data, ok := c.get(key)
	if !ok || data == "" {
		return false
	}
	newData, ok := c.toJson(info, "value")
	if !ok {
		return false
	}
	c.logger.V(4).Info("Comparing old and new date",
		"old", data,
		"new", newData,
		"result", data == newData)
	return data == newData
}

func (c *GoliveCache) SetIfOutdated(key interface{}, info interface{}) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isUpdateToDate(key, info) {
		return false
	} else {
		c.set(key, info)
		return true
	}
}

func (c *GoliveCache) toJson(element interface{}, fieldName string) (string, bool) {
	jsonKey, err := json.Marshal(element)
	if err != nil {
		c.logger.Info("Not able to compute json for cache", "field", fieldName)
		return "", false
	}
	return string(jsonKey), err == nil
}
