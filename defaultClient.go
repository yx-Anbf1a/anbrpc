package main

import (
	"context"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"sync"
)

type DClient struct {
	discovery Discovery
	opt       *Option
	mu        sync.Mutex
	clients   map[string]*Client
}

var _ io.Closer = (*DClient)(nil)

func (dc *DClient) Close() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	for _, client := range dc.clients {
		_ = client.Close()
	}
	_ = dc.discovery.Close()
	return nil
}

func NewDClient(d Discovery, opt *Option) *DClient {
	dc := &DClient{
		discovery: d,
		opt:       opt,
		clients:   make(map[string]*Client),
	}
	return dc
}

func (dc *DClient) WithBalancer(b Balancer) {
	dc.discovery.SetBalancer(b)
	return
}

func (dc *DClient) dial(rpcAddr string) (*Client, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	c, ok := dc.clients[rpcAddr]
	if ok && !c.IsAlive() {
		_ = c.Close()
		delete(dc.clients, rpcAddr)
		c = nil
	}
	if c == nil {
		var err error
		c, err = DDial(rpcAddr, dc.opt)
		if err != nil {
			return nil, err
		}
		dc.clients[rpcAddr] = c
	}
	return c, nil
}

func (dc *DClient) call(ctx context.Context, rpcAddr, serviceMethod string, args, reply proto.Message) error {
	client, err := dc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

func (dc *DClient) Call(ctx context.Context, serviceMethod string, args, reply proto.Message) error {
	rpcAddr := dc.discovery.GetService()
	log.Printf("call %s on %s", serviceMethod, rpcAddr)
	return dc.call(ctx, rpcAddr, serviceMethod, args, reply)
}
