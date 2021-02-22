package main

import "freeminerserver/server"

func main()  {
	newServer :=server.New("pool.json")
	newServer.Start()
	select {

	}
}