package server

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"freeminerserver/proxy"
	"freeminerserver/storage"
)

type PoolServer struct {
	PoolConfig proxy.Config
	Backend *storage.RedisClient
}

func New(path string)*PoolServer{

	newServer :=new(PoolServer)

	configFileName := path
	configFileName, _ = filepath.Abs(configFileName)
	log.Printf("Loading config: %v", configFileName)

	configFile, err := os.Open(configFileName)
	if err != nil {
		log.Fatal("File error: ", err.Error())
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err := jsonParser.Decode(&newServer.PoolConfig); err != nil {
		log.Fatal("Config error: ", err.Error())
	}

	rand.Seed(time.Now().UnixNano())

	if newServer.PoolConfig.Threads > 0 {
		runtime.GOMAXPROCS(newServer.PoolConfig.Threads)
		log.Printf("Running with %v threads", newServer.PoolConfig.Threads)
	}

	newServer.Backend = storage.NewRedisClient(&newServer.PoolConfig.Redis, newServer.PoolConfig.Coin)
	pong, err := newServer.Backend.Check()
	if err != nil {
		log.Printf("Can't establish connection to cbackend: %v", err)
	} else {
		log.Printf("cbackend check reply: %v", pong)
	}
	return newServer
}

func (this *PoolServer) Start()  {
	go func() {
		proxy.NewProxy( &this.PoolConfig, this.Backend)
	}()
}
