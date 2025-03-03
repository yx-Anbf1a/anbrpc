package main

import (
	"log"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	// TODO
	endpoints := []string{"127.0.0.1:2379"}
	d := NewServiceDiscovery(endpoints)
	defer d.Close()
	d.WatchService("/test")
	//d.WatchService("/gRPC/")
	for {
		select {
		case <-time.Tick(10 * time.Second):
			log.Println(d.GetAllService())
		}
	}
}
