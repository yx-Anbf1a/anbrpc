package balancer

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type BalanceMode int

var (
	ErrNoServer      = errors.New("no server")
	ErrNoBalanceMode = errors.New("no balance mode")
)

const (
	RandomMode     BalanceMode = iota // 随机
	RoundRobinMode                    // 轮训
)

type BalancerBuilder interface {
	Build() (Balancer, error)
}

type Balancer interface {
	Pick(srvKey []string) (string, error)
}

type RandomBalancerBuild struct{}

func NewRandomBalancerBuild() *RandomBalancerBuild {
	return &RandomBalancerBuild{}
}

func (b *RandomBalancerBuild) Build() (Balancer, error) {
	return NewRandomBalancer(), nil
}

type RandomBalancer struct {
	mu sync.RWMutex
	r  *rand.Rand // generate random number
}

func NewRandomBalancer() *RandomBalancer {
	rb := &RandomBalancer{
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	return rb
}

func (b *RandomBalancer) Pick(srvKey []string) (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(srvKey) == 0 {
		return "", ErrNoServer
	}
	return srvKey[b.r.Intn(len(srvKey))], nil
}

type RoundRobinBalancerBuild struct{}

func NewRoundRobinBalancerBuild() *RoundRobinBalancerBuild {
	return &RoundRobinBalancerBuild{}
}

func (b *RoundRobinBalancerBuild) Build() (Balancer, error) {
	return NewRoundRobinBalancer(), nil
}

type RoundRobinBalancer struct {
	r     *rand.Rand // generate random number
	mu    sync.RWMutex
	index uint32
}

func (rb *RoundRobinBalancer) Pick(srvKey []string) (string, error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	n := uint32(len(srvKey))
	if n == 0 {
		return "", ErrNoServer
	}
	key := rb.index % n
	rb.index = (rb.index + 1) % n
	return srvKey[key], nil
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	rb := &RoundRobinBalancer{
		index: uint32(rand.Intn(math.MaxInt - 1)),
	}
	return rb
}
