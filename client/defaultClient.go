package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/yx-Anbf1a/anbrpcv/balancer"
	"github.com/yx-Anbf1a/anbrpcv/discovery"
	"github.com/yx-Anbf1a/anbrpcv/option"
	"google.golang.org/protobuf/proto"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type DClient struct {
	discovery discovery.Discovery
	opt       *option.Option

	mu        sync.Mutex
	clients   map[string]*Client
	endpoints []string
	bl        balancer.Balancer

	pools map[string]Pool

	InitialCap int
	//最大并发存活连接数
	MaxCap int
	//最大空闲连接
	MaxIdle     int
	IdleTimeout time.Duration
}

var _ io.Closer = (*DClient)(nil)

func (dc *DClient) Close() error {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	for _, client := range dc.clients {
		_ = client.Close()
	}
	_ = dc.discovery.Close()
	for _, pool := range dc.pools {
		pool.Release()
	}
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
		endpoints:   endpoints,
		opt:         op,
		clients:     make(map[string]*Client),
		pools:       make(map[string]Pool),
		InitialCap:  2,
		MaxCap:      5,
		MaxIdle:     4,
		IdleTimeout: 15 * time.Second,
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
		parts := strings.Split(rpcAddr, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("rpc client err: wrong format '%s', expect protocol@addr", rpcAddr)
		}
		protocol, addr := parts[0], parts[1]

		pool, ok := dc.pools[rpcAddr]
		if !ok {
			poolOpt := PoolOption{
				InitialCap:  dc.InitialCap,
				MaxIdle:     dc.MaxIdle,
				MaxCap:      dc.MaxCap,
				IdleTimeout: dc.IdleTimeout,
				Network:     protocol,
				Address:     addr,
			}
			pool, _ = NewChannelPool(&poolOpt)
			dc.pools[rpcAddr] = pool
		}

		conn, err := pool.Get()
		if err != nil {
			return nil, err
		}
		c, err = DDial(protocol, conn.(net.Conn), dc.opt)
		_ = pool.Put(conn)

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
