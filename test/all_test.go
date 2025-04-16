package test

import (
	"context"
	bl "github.com/yx-Anbf1a/anbrpcv/balancer"
	"github.com/yx-Anbf1a/anbrpcv/client"
	"github.com/yx-Anbf1a/anbrpcv/server"
	"github.com/yx-Anbf1a/anbrpcv/test_service"
	"log"
	_ "net/http/pprof"
	"strconv"
	"sync"
	"testing"
	"time"
)

func startServer(service interface{}, endpoints []string, key string, wg *sync.WaitGroup) {
	svr := server.NewServer(":0")
	_ = svr.Register(server.RegisterConfig{
		Endpoints:   endpoints,
		ServiceName: key,
	}, service)
	svr.Run()

	wg.Done()
}

func foo(xc *client.DClient, ctx context.Context, serviceMethod string, args *test_service.FBooArgs) {
	var reply test_service.FBooReply
	var err error

	err = xc.Call(ctx, serviceMethod, args, &reply)
	if err != nil {
		log.Printf("%s error: %v", serviceMethod, err)
	} else {
		log.Printf("%s success: %d + %d = %v", serviceMethod, args.Num1, args.Num2, reply.Num)
	}
}

func call(endpoints []string, prefix string, balancer bl.Balancer) {
	//builder := NewRandomBalancerBuild()
	//balancer, _ := builder.Build()
	//d := discovery.NewServiceDiscovery(endpoints, balancer)
	//d := NewServiceDiscovery(endpoints)
	//_ = d.WatchService(prefix)
	xc := client.NewDClient(endpoints)
	xc.SetBalancer(balancer)
	xc.Watch(prefix)

	defer func() {
		log.Println("XC close !")
		err := xc.Close()
		if err != nil {
			log.Println(err)
		}
	}()
	// send request & receive response
	var wg sync.WaitGroup
	ctx := context.Background()
	ctx, _ = context.WithTimeout(ctx, time.Second*5)

	i := 1
	loop := true
	for loop {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := test_service.FBooArgs{Num1: int32(i), Num2: int32(i * i)}
			//foo(xc, context.Background(), "Boo.Sum", &Args{Num1: i, Num2: i * i})
			foo(xc, context.Background(), "FBoo.Sum", &args)
			foo(xc, context.Background(), "FBoo.Sleep", &args)
		}(i)

		select {
		case <-ctx.Done():
			loop = false
		default:
			time.Sleep(time.Millisecond * 100)
			i++
		}
	}
	wg.Wait()
}

func TestNewServer(t *testing.T) {
	endpoints := []string{"localhost:2379"}

	log.SetFlags(0)
	var wg sync.WaitGroup
	var boo test_service.FBoo
	for i := 0; i < 3; i++ {
		wg.Add(1)
		key := "/boo/node" + strconv.Itoa(i)
		go startServer(&boo, endpoints, key, &wg)
	}
	//wg.Add(1)
	//time.Sleep(time.Second * 10)
	//go startServer(&boo, endpoints, "/boo/node2", &wg)
	wg.Wait()
	select {
	//case <-time.After(time.Second * 20):
	//	t.Log("test timeout")
	}
}

func TestRandomBalancerBuild(t *testing.T) {
	endpoints := []string{"localhost:2379"}
	time.Sleep(time.Second)
	call(endpoints, "/boo", bl.NewRoundRobinBalancer())
}

func TestRoundRobinBalancerBuild(t *testing.T) {

	endpoints := []string{"localhost:2379"}
	time.Sleep(time.Second)
	call(endpoints, "/boo", bl.NewRoundRobinBalancer())

	//ip := "127.0.0.1:6069"
	//if err := http.ListenAndServe(ip, nil); err != nil {
	//	fmt.Printf("start pprof failed on %s\n", ip)
	//}
	//select {}
}
