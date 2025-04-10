package main

import (
	"context"
	"fmt"
	"log"
	"myRPC/test_service"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

type Boo int

type Args struct{ Num1, Num2 int }

func (b Boo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (b Boo) Sleep(args Args, reply *int) error {
	time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

func startServer(endpoints []string, key string, wg *sync.WaitGroup) {
	var boo test_service.FBoo
	l, _ := net.Listen("tcp", ":0")
	r, _ := NewServiceRegister(endpoints, key, "tcp@"+l.Addr().String(), 20)

	server := NewServer()
	server.WithRegister(r)
	_ = server.Register(&boo)

	wg.Done()
	server.Accept(l)
}

func foo(xc *DClient, ctx context.Context, serviceMethod string, args *test_service.FBooArgs) {
	var reply test_service.FBooReply
	var err error

	err = xc.Call(ctx, serviceMethod, args, &reply)
	if err != nil {
		log.Printf("%s error: %v", serviceMethod, err)
	} else {
		log.Printf("%s success: %d + %d = %v", serviceMethod, args.Num1, args.Num2, reply.Num)
	}
}

func call(endpoints []string, prefix string, builder BalancerBuilder) {
	//builder := NewRandomBalancerBuild()
	balancer, _ := builder.Build()

	d := NewServiceDiscovery(endpoints, balancer)
	//d := NewServiceDiscovery(endpoints)
	_ = d.WatchService(prefix)
	xc := NewDClient(d, DefaultOption)

	defer func() {
		log.Println("XC close !")
		err := xc.Close()
		if err != nil {
			log.Println(err)
		}
	}()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := test_service.FBooArgs{Num1: int32(i), Num2: int32(i * i)}
			//foo(xc, context.Background(), "Boo.Sum", &Args{Num1: i, Num2: i * i})
			foo(xc, context.Background(), "FBoo.Sum", &args)
			//time.Sleep(time.Second)
			foo(xc, context.Background(), "FBoo.Sleep", &args)
		}(i)
	}
	wg.Wait()
}

func TestNewServer(t *testing.T) {
	endpoints := []string{"localhost:2379"}

	log.SetFlags(0)
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		key := "/test/node" + strconv.Itoa(i)
		go startServer(endpoints, key, &wg)
	}
	wg.Add(1)
	time.Sleep(time.Second * 10)
	go startServer(endpoints, "/test/node2", &wg)
	wg.Wait()
	select {
	//case <-time.After(time.Second * 20):
	//	t.Log("test timeout")
	}
}

func TestRandomBalancerBuild(t *testing.T) {
	endpoints := []string{"localhost:2379"}
	time.Sleep(time.Second)
	call(endpoints, "/test", NewRandomBalancerBuild())
}

func TestRoundRobinBalancerBuild(t *testing.T) {
	endpoints := []string{"localhost:2379"}
	time.Sleep(time.Second)
	call(endpoints, "/test", NewRoundRobinBalancerBuild())
	//time.Sleep(time.Second)
	//call(endpoints, "/test", NewRoundRobinBalancerBuild())
	//ch := make(chan int)
	//for i := 0; i < 10; i++ {
	//	ch <- i
	//}
	//go func() {
	//	for num := range ch {
	//		log.Println("num:", num)
	//	}
	//}()
}

type Shape interface {
	Hello()
}

type Hi struct{}

func (h *Hi) Hello() {
	fmt.Println("hello")
}

func HHi(h interface{}) {
	h.(Shape).Hello()
}

func TestInterface(t *testing.T) {
	var h interface{} = &Hi{}
	HHi(h)
}
