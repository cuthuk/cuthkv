package config

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	Server struct {
		Host string
		Port string
	}

	Options struct {
		TimeoutGC         int
		DisabledGC        bool
		FilterKeysWorkers int
	}

	Cluster struct {
		StoreList          []string
		StatCrawlerTimeout int
	}
}

var Cfg Config
var selfName string

func InitCfg() {
	// get Selfname
	selfName = filepath.Base(os.Args[0])
	// Reads info from config file
	_, err := os.Stat(selfName + ".toml")
	if err != nil {
		log.Fatal("Config file is missing: ", selfName+".toml")
	}

	if _, err := toml.DecodeFile(selfName+".toml", &Cfg); err != nil {
		log.Fatal(err)
	}
}
