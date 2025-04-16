package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yx-Anbf1a/anbrpc/codec"
	"github.com/yx-Anbf1a/anbrpc/option"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Call struct {
	Seq           uint64 // Call序列号
	ServiceMethod string // Service.Method
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call // 监听关闭
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc       codec.Codec // 客户端编解码器
	opt      *option.Option
	sending  sync.Mutex       // 保证请求并发安全
	header   codec.Header     // 请求头
	mu       sync.Mutex       // 自己并发操作安全
	seq      uint64           // 客户端序列号
	pending  map[uint64]*Call // 存储已经发送但未完成的请求
	closing  bool             // 用户主动关闭
	shutdown bool             // 服务器关闭
}

type ClientResult struct {
	Client *Client
	Err    error
}

var ErrShutdown = errors.New("connection is shut down")

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

// IsAlive 客户端是否存活
func (c *Client) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

func (c *Client) RegisterCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 判断连接是否关闭
	if c.shutdown || c.closing {
		return 0, ErrShutdown
	}
	call.Seq = c.seq
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

// RemoveCall的作用是删除已经完成的请求并获取这个请求
func (c *Client) RemoveCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// TerminateCalls 关闭所有未完成的请求
func (c *Client) TerminateCalls(err error) {
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

func (c *Client) receive() {

	//errChan := make(chan error, 1)
	var err error
	//timeoutChan := make(chan struct{}, 1)

	for err == nil {
		var h codec.Header
		if err = c.cc.ReadHeader(&h); err != nil {
			//errChan <- err
			break
		}
		call := c.RemoveCall(h.Seq)
		switch {
		case call == nil:
			err = c.cc.ReadBody(nil, h.BodySize)
		case h.Error != "":
			call.Error = errors.New(h.Error)
			err = c.cc.ReadBody(nil, h.BodySize)
			call.done()
		default:
			// 正常就读取请求，标记完成
			err = c.cc.ReadBody(call.Reply, h.BodySize)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	// 接收请求遇到异常关闭
	c.TerminateCalls(err)
}

func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()
	if c.shutdown || c.closing {
		return
	}
	// 注册一个Call
	seq, err := c.RegisterCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = call.Seq
	c.header.Error = ""
	if err = c.cc.Write(&c.header, call.Args); err != nil {
		call := c.RemoveCall(seq)
		if call != nil {
			call.Error = errors.New("writing request: " + err.Error())
			call.done()
		}
	}
}

func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		panic("rpc client: done has no cap")
	}
	call := &Call{
		Seq:           0,
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}

func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := c.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		c.RemoveCall(call.Seq)
		return errors.New("rpc client: call failed " + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}
}

type newClientFunc func(conn net.Conn, opt *option.Option) (*Client, error)

func dial(f newClientFunc, conn net.Conn, opts ...*option.Option) (client *Client, err error) {
	opt, err := option.ParseOption(opts...)
	if err != nil {
		return nil, err
	}

	// 连接池...
	//conn, err := net.DialTimeout(network, address, opt.ConnectTimeOut)
	//
	//if err != nil {
	//	return nil, err
	//}
	defer func() {
		if err != nil && conn != nil {
			_ = conn.Close()
		}
	}()
	ch := make(chan ClientResult)
	go func() {
		client, err := f(conn, opt)
		ch <- ClientResult{
			Client: client,
			Err:    err,
		}
	}()
	if opt.ConnectTimeOut == 0 {
		result := <-ch
		return result.Client, result.Err
	}
	select {
	case <-time.After(opt.ConnectTimeOut):
		return nil, errors.New("rpc client: connect timeout")
	case result := <-ch:
		return result.Client, result.Err
	}
}

func NewClient(conn net.Conn, opt *option.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := errors.New("invalid codec type")
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	// 发送请求设置
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt)
}

func newClientCodec(cc codec.Codec, opt *option.Option) (*Client, error) {
	// 发送请求设置
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	// 开启接收请求
	go client.receive()
	return client, nil
}

func NewHTTPClient(conn net.Conn, opt *option.Option) (*Client, error) {
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/2.0\n\n", option.DefaultRPCPath))

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == option.Connected {
		return NewClient(conn, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

func DialHTTP(conn net.Conn, opts ...*option.Option) (*Client, error) {
	return dial(NewHTTPClient, conn, opts...)
}

func Dial(conn net.Conn, opts ...*option.Option) (client *Client, err error) {
	return dial(NewClient, conn, opts...)
}

func DDial(protocol string, conn net.Conn, opts ...*option.Option) (*Client, error) {

	switch protocol {
	case "http":
		return DialHTTP(conn, opts...)
	default:
		// tcp, unix or other transport protocol
		return Dial(conn, opts...)
	}
}
