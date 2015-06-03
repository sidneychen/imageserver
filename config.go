package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type CacheConfig struct {
	Addr string `json:"addr,omitempty"`
	Type string `json:"type, omitempty"`
}

type Config struct {
	Listen string       `json:"listen"`
	cc     *CacheConfig `json:"cache"`
}

func NewCacheConfigDefault() *CacheConfig {
	return &CacheConfig{
		Addr: "127.0.0.1:11211",
		Type: "memcache",
	}
}

func NewConfigDefault() *Config {
	cfg := new(Config)
	cfg.Listen = ":8080"
	cfg.cc = NewCacheConfigDefault()
	return cfg
}

func NewConfigFromFile(filename string) *Config {
	data, _ := ioutil.ReadFile(filename)
	return NewConfig(data)
}

func NewConfig(data []byte) *Config {
	cfg := NewConfigDefault()
	err := json.Unmarshal(data, cfg)
	if err != nil {
		log.Fatal("config is not a valid json")
	}
	return cfg
}
