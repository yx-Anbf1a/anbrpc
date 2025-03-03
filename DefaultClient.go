package main

import (
	"context"
	"io"
	"sync"
)

type DClient struct {
	d       Discovery
	b       Balancer
	opt     *Option
	mu      sync.Mutex
	clients map[string]*Client
}

var _ io.Closer = (*DClient)(nil)

func (dc *DClient) Close() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	for _, client := range dc.clients {
		_ = client.Close()
	}
	return nil
}

func newDClient(d Discovery, b Balancer, opt *Option) *DClient {
	return &DClient{
		d:       d,
		b:       b,
		opt:     opt,
		clients: make(map[string]*Client),
	}
}

func (dc *DClient) dial(rpcAddr string) (*Client, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	client, ok := dc.clients[rpcAddr]
	if ok && !client.IsAlive() {
		_ = client.Close()
		delete(dc.clients, rpcAddr)
		client = nil
	}
	if client == nil {
		client, err := DDial(rpcAddr, dc.opt)
		if err != nil {
			return nil, err
		}
		dc.clients[rpcAddr] = client
	}
	return client, nil
}

func (dc *DClient) call(ctx context.Context, rpcAddr, serviceMethod string, args, reply interface{}) error {
	client, err := dc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

func (dc *DClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := dc.b.Pick()
	if err != nil {
		return err
	}
	return dc.call(ctx, rpcAddr, serviceMethod, args, reply)
}

func (dc *DClient) SetBalancer(b Balancer) {
	dc.b = b
}
