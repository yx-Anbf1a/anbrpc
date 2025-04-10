package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"myRPC/codec"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Server struct {
	ServerMap sync.Map // 保证并发安全
	register  *ServiceRegister
}

func NewServer() *Server {
	s := &Server{}
	return s
}

var DefaultServer = NewServer()

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

func WithRegister(register *ServiceRegister) {
	DefaultServer.WithRegister(register)
}

func (s *Server) WithRegister(register *ServiceRegister) {
	s.register = register
}

func (s *Server) Accept(lis net.Listener) {
	if s.register == nil {
		log.Println("register is nil")
		return
	}
	go s.register.ListenLeaseRespChan()
	//defer s.register.Close()
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
			return
		}
		go s.ServeConn(conn)
	}
}

// ServeConn 处理连接
/*
	1. 先检查请求头，确认请求头是否合法，不合法则关闭连接
	2. 根据请求头中的协议类型，处理连接
*/
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("decode option error:", err)
		return
	}
	if opt.MagicNumber != DefaultOption.MagicNumber {
		log.Printf("invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("invalid codec type %s", opt.CodecType)
		return
	}
	s.ServeCodec(f(conn), &opt)
}

var invalidRequest = struct{}{}

func (s *Server) ServeCodec(cc codec.Codec, opt *Option) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, nil, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg, opt.HandleTimeOut)
	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
	mtype        *MethodType
	svc          *Service
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {

	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{
		h: h,
	}
	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return nil, err
	}
	req.argv = req.mtype.newArgs()
	req.replyv = req.mtype.newReply()
	// 读取请求内容

	// 需要用指针取读取内容
	//if req.argv.Kind() != reflect.Ptr {
	//	argvi = req.argv.Addr().Interface().(proto.Message)
	//}
	// 确保 req.argv 包含的值实现了 proto.Message 接口
	if req.argv.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("argument type must be a pointer to a struct implementing proto.Message")
	}
	argvi := req.argv.Interface()
	if err = cc.ReadBody(argvi); err != nil {
		log.Println("read body error:", err)
		return nil, err
	}
	return req, nil
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header

	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) findService(serviceMethod string) (svc *Service, mtype *MethodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	// Service.Method
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	// 获取服务
	svci, ok := s.ServerMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*Service)
	// 获取方法
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{}, 1)
	sent := make(chan struct{}, 1)
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, nil, sending)
			sent <- struct{}{}
			return
		}
		s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
		return
	}()
	if timeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-called:
		<-sent
	case <-time.After(timeout):
		req.h.Error = "rpc server: request handle timeout"
		s.sendResponse(cc, req.h, nil, sending)
	}
}

func (s *Server) Register(rcvr interface{}) error {
	service := newService(rcvr)
	if _, dup := s.ServerMap.LoadOrStore(service.name, service); dup {
		return errors.New("rpc: service already defined: " + service.name)
	}
	return nil
}

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

const (
	connected        = "200 Connected to Gee RPC"
	defaultRPCPath   = "/_geeprc_"
	defaultDebugPath = "/debug/geerpc"
)

func (server *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, server)
	http.Handle(defaultDebugPath, debugHTTP{server})
	log.Println("rpc server debug path:", defaultDebugPath)
}

// ServeHTTP implements an http.Handler that answers RPC requests.
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(conn, "HTTP/2.0 "+connected+"\n\n")
	server.ServeConn(conn)
}
