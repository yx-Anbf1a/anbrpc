package main

import (
	"context"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/clientv3"
	"log"
	"sync"
	"time"
)

type BalanceMode int

const (
	RandomSelect     BalanceMode = iota // 随机
	RoundRobinSelect                    // 轮训
)

type Discovery interface {
	Refresh() error                         // 从注册中心更新服务列表
	Update(servers map[string]string) error // 手动更新
	GetService(key string) string           // 根据负载均衡策略，选择一个服务实例
	GetAllService() []string
}

type ServerDiscovery struct {
	// 服务列表
	servers map[string]string
	// 负载均衡策略
	mode BalanceMode
	// 上次选择的服务实例
	lastIndex int
	// etcd 客户端
	cli *clientv3.Client
	mu  sync.Mutex
}

// NewServiceDiscovery  新建发现服务
func NewServiceDiscovery(endpoints []string) *ServerDiscovery {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	return &ServerDiscovery{
		cli:     cli,
		servers: make(map[string]string),
	}
}

// watcher 监听前缀
func (s *ServerDiscovery) watcher(prefix string) {
	rch := s.cli.Watch(context.Background(), prefix, clientv3.WithPrefix())
	log.Printf("watching prefix:%s now...", prefix)
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch ev.Type {
			case mvccpb.PUT: //修改或者新增
				s.SetServices(string(ev.Kv.Key), string(ev.Kv.Value))
			case mvccpb.DELETE: //删除
				s.DelServiceList(string(ev.Kv.Key))
			}
		}
	}
}

// WatchService 初始化服务列表和监视
func (s *ServerDiscovery) WatchService(prefix string) error {
	//根据前缀获取现有的key
	resp, err := s.cli.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return err
	}
	for _, ev := range resp.Kvs {
		s.SetServices(string(ev.Key), string(ev.Value))
	}
	//监视前缀，修改变更的server
	go s.watcher(prefix)
	return nil
}

// SetServiceList 新增服务地址
func (s *ServerDiscovery) SetServices(key, val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers[key] = val
	log.Println("put key :", key, "val:", val)
}

func (s *ServerDiscovery) Refresh() error {
	//TODO
	return nil
}

func (s *ServerDiscovery) Update(servers map[string]string) error {
	//TODO
	s.mu.Lock()
	defer s.mu.Unlock()
	s.servers = servers
	return nil
}

// DelServiceList 删除服务地址
func (s *ServerDiscovery) DelServiceList(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.servers, key)
	log.Println("del key:", key)
}

// GetServices 获取服务地址
func (s *ServerDiscovery) GetService(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	addr := s.servers[key]
	return addr
}
func (s *ServerDiscovery) GetAllService() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	addrs := make([]string, 0)

	for _, v := range s.servers {
		addrs = append(addrs, v)
	}
	return addrs
}

// Close 关闭服务
func (s *ServerDiscovery) Close() error {
	return s.cli.Close()
}
