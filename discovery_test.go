package main

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func TestDiscovery(t *testing.T) {
	// TODO
	endpoints := []string{"127.0.0.1:2379"}
	builder := NewRandomBalancerBuild()
	balancer, _ := builder.Build()

	d := NewServiceDiscovery(endpoints, balancer)
	defer d.Close()
	d.WatchService("/test")
	//d.WatchService("/gRPC/")
	for {
		select {
		case <-time.Tick(10 * time.Second):
			log.Println(d.GetService())
		}
	}
}

func TestAccept(t *testing.T) {
	a := [10]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	b := a[:3]
	fmt.Println(len(b), cap(b))
	b = append(b, 11, 12, 13, 14)
	fmt.Println(len(b), cap(b))
	fmt.Println(a, b)
}
