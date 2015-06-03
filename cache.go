package main

import (

	//	"github.com/kklis/gomemcache"
	"github.com/bradfitz/gomemcache/memcache"
)

type Cache interface {
	Set(string, []byte, int32) error
	Get(string) ([]byte, error)
	Delete(string) error
}

type Memcache struct {
	mc *memcache.Client
}

func NewMemcache(addr string) *Memcache {
	return &Memcache{memcache.New(addr)}
}

func (m *Memcache) Set(key string, data []byte, expiration int32) error {
	item := &memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: expiration,
	}
	return m.mc.Set(item)
}

func (m *Memcache) Get(key string) ([]byte, error) {
	item, err := m.mc.Get(key)
	if err != nil {
		return nil, err
	}
	return item.Value, nil
}

func (m *Memcache) Delete(key string) error {
	return m.mc.Delete(key)
}
