package main

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrNoServer      = errors.New("no server")
	ErrNoBalanceMode = errors.New("no balance mode")
)

const (
	RandomMode     BalanceMode = iota // 随机
	RoundRobinMode                    // 轮训
)

type BalancerBuilder interface {
	Build(mode BalanceMode, srvKey []string) (Balancer, error)
	ShiftMode(mode BalanceMode, balancer Balancer) (Balancer, error)
}

type Balancer interface {
	Pick() (string, error)
	SetServers(srvKey []string)
	GetServers() []string
}

type RandomBalancerBuild struct{}

func NewRandomBalancerBuild() *RandomBalancerBuild {
	return &RandomBalancerBuild{}
}

func (b *RandomBalancerBuild) Build(mode BalanceMode, srvKey []string) (Balancer, error) {
	if mode != RandomMode {
		return nil, ErrNoBalanceMode
	}
	return newRandomBalancer(srvKey), nil
}

func (b *RandomBalancerBuild) ShiftMode(mode BalanceMode, balancer Balancer) (Balancer, error) {
	if mode != RoundRobinMode {
		return nil, ErrNoBalanceMode
	}
	return newRandomBalancer(balancer.GetServers()), nil
}

type RandomBalancer struct {
	mu     sync.RWMutex
	r      *rand.Rand // generate random number
	srvKey []string   // 存放的是etcd里的服务器key
}

func newRandomBalancer(srvKey []string) *RandomBalancer {
	rb := &RandomBalancer{
		srvKey: srvKey,
		r:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return rb
}

func (b *RandomBalancer) Pick() (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.srvKey) == 0 {
		return "", ErrNoServer
	}
	return b.srvKey[b.r.Intn(len(b.srvKey))], nil
}

func (b *RandomBalancer) SetServers(srvKey []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.srvKey = srvKey
}

func (b *RandomBalancer) GetServers() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.srvKey
}

type RoundRobinBalancerBuild struct{}

func NewRoundRobinBalancerBuild() *RoundRobinBalancerBuild {
	return &RoundRobinBalancerBuild{}
}

func (b *RoundRobinBalancerBuild) Build(mode BalanceMode, srvKey []string) (Balancer, error) {
	if mode != RoundRobinMode {
		return nil, ErrNoBalanceMode
	}
	return newRoundRobinBalancer(srvKey), nil
}

func (b *RoundRobinBalancerBuild) ShiftMode(mode BalanceMode, balancer Balancer) (Balancer, error) {
	if mode != RoundRobinMode {
		return nil, ErrNoBalanceMode
	}
	return newRoundRobinBalancer(balancer.GetServers()), nil
}

type RoundRobinBalancer struct {
	mu     sync.RWMutex
	srvKey []string // 存放的是etcd里的服务器key
	index  uint32
}

func (rb *RoundRobinBalancer) Pick() (string, error) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	n := uint32(len(rb.srvKey))
	if n == 0 {
		return "", ErrNoServer
	}
	idx := atomic.AddUint32(&rb.index, 1)
	return rb.srvKey[idx%n], nil
}

func (rb *RoundRobinBalancer) SetServers(srvKey []string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.srvKey = srvKey
	rb.index = uint32(rand.Intn(len(srvKey)))
}

func (rb *RoundRobinBalancer) GetServers() []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.srvKey
}

func newRoundRobinBalancer(srvKey []string) *RoundRobinBalancer {
	rb := &RoundRobinBalancer{
		srvKey: srvKey,
	}
	rb.index = uint32(rand.Intn(len(srvKey)))
	return rb
}
