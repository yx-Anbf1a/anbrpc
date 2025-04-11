package client

import (
	"context"
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
	"myRPC/balancer"
	"myRPC/discovery"
	"myRPC/option"
	"sync"
	"time"
)

type DClient struct {
	discovery discovery.Discovery
	opt       *option.Option
	mu        sync.Mutex
	clients   map[string]*Client
	services  map[string][]string // 存储服务地址
	endpoints []string
	bl        balancer.Balancer
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

func NewDClient(endpoints []string, opt ...*option.Option) *DClient {
	var op *option.Option
	if len(opt) == 0 {
		op = option.DefaultOption
	} else {
		op = opt[0]
	}
	dc := &DClient{
		endpoints: endpoints,
		opt:       op,
		clients:   make(map[string]*Client),
	}
	dc.bl = balancer.NewRandomBalancer()
	dc.newDiscovery(dc.endpoints, dc.bl)
	return dc
}

func (dc *DClient) newDiscovery(endpoints []string, bl balancer.Balancer) {
	dc.discovery = discovery.NewServiceDiscovery(endpoints, bl)
}

func (dc *DClient) Watch(prefix string) {
	_ = dc.discovery.WatchService(prefix)
}

func (dc *DClient) SetBalancer(b balancer.Balancer) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
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
	ctx, _ = context.WithTimeout(ctx, time.Second*3)
	for rpcAddr == "" {
		rpcAddr = dc.discovery.GetService()
		//log.Println("wait for service...")
		select {
		case <-ctx.Done():
			return errors.New("no expect service")
		default:
		}
	}
	//log.Printf("call %s on %s", serviceMethod, rpcAddr)
	return dc.call(ctx, rpcAddr, serviceMethod, args, reply)
}
