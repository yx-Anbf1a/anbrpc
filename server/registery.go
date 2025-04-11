package server

import (
	"context"
	"errors"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"log"
	"time"
)

type RegisterConfig struct {
	Endpoints   []string
	ServiceName string // Key
	Host        string // Value
	Lease       int64
}

type ServiceRegister struct {
	cli     *clientv3.Client //etcd client
	leaseID clientv3.LeaseID //租约ID
	//租约keepalieve相应chan
	keepAliveChan <-chan *clientv3.LeaseKeepAliveResponse
	key           string //key
	val           string //value
	logger        *zap.Logger
}

// NewServiceRegister 新建注册服务
func NewServiceRegister(config RegisterConfig) (*ServiceRegister, error) {
	if config.Host == "" || config.ServiceName == "" || len(config.Endpoints) == 0 {
		return nil, errors.New("请填入正确RegisterConfig参数")
	}
	if config.Lease == 0 {
		config.Lease = 20
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   config.Endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	ser := &ServiceRegister{
		cli: cli,
		key: config.ServiceName,
		val: config.Host,
	}

	//申请租约设置时间keepalive
	if err = ser.putKeyWithLease(config.Lease); err != nil {
		return nil, err
	}
	return ser, nil
}

// 设置租约
func (s *ServiceRegister) putKeyWithLease(lease int64) error {
	//设置租约时间
	resp, err := s.cli.Grant(context.Background(), lease)
	if err != nil {
		return err
	}
	//注册服务并绑定租约
	_, err = s.cli.Put(context.Background(), s.key, s.val, clientv3.WithLease(resp.ID))
	if err != nil {
		return err
	}
	//设置续租 定期发送需求请求
	leaseRespChan, err := s.cli.KeepAlive(context.Background(), resp.ID)

	if err != nil {
		return err
	}
	s.leaseID = resp.ID
	log.Println(s.leaseID)
	s.keepAliveChan = leaseRespChan
	log.Printf("Put key:%s  val:%s  success!", s.key, s.val)
	return nil
}

// ListenLeaseRespChan 监听 续租情况
func (s *ServiceRegister) ListenLeaseRespChan() {
	for leaseKeepResp := range s.keepAliveChan {
		//log.Println("续约成功", leaseKeepResp)
		s.logger.Info("续约成功", zap.Any("leaseKeepResp", leaseKeepResp.String()))
	}
	//log.Println("关闭续租")
	s.logger.Info("关闭续租")
}

// Close 注销服务
func (s *ServiceRegister) Close() error {
	//撤销租约
	if _, err := s.cli.Revoke(context.Background(), s.leaseID); err != nil {
		return err
	}
	//log.Println("撤销租约")
	s.logger.Info("撤销租约")
	return s.cli.Close()
}
