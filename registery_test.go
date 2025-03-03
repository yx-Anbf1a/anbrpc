package main

import (
	"log"
	"testing"
)

func TestNewServiceRegister(t *testing.T) {
	var endpoints = []string{"localhost:2379"}
	ser, err := NewServiceRegister(endpoints, "/test/node1", "localhost:8000", 5)
	ser2, err := NewServiceRegister(endpoints, "/test/node2", "localhost:9000", 5)
	if err != nil {
		log.Fatalln(err)
	}
	//监听续租相应chan
	go ser.ListenLeaseRespChan()
	go ser2.ListenLeaseRespChan()
	select {
	//case <-time.After(20 * time.Second):
	//	ser.Close()
	}
}
