package main

import "github.com/panglove/freeminerserver/server"

func main()  {
	newServer :=server.New("pool.json")
	newServer.Start()
	select {

	}
}