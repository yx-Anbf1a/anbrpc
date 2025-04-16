package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yx-Anbf1a/myRPC/codec"
	"github.com/yx-Anbf1a/myRPC/logger"
	"github.com/yx-Anbf1a/myRPC/option"
	"go.uber.org/zap"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Server struct {
	ServiceMap sync.Map // 保证并发安全
	Host       string
	register   *ServiceRegister
	l          net.Listener
	mu         sync.Mutex
	logger     *zap.Logger
}

func NewServer(address string) *Server {

	//server.WithRegister(r)
	l, _ := net.Listen("tcp", address)
	//r, _ := NewServiceRegister(endpoints, key, "tcp@"+l.Addr().String(), 20)
	server := &Server{}
	server.l = l
	server.logger, _ = logger.InitLogger("server.log", "dev")
	server.Host = "tcp@" + l.Addr().String()
	return server
}

var defaultServer *Server
var once sync.Once

func DefaultServer() *Server {
	once.Do(func() {
		defaultServer = NewServer(":0")
	})
	return defaultServer
}

func (s *Server) Run() {
	s.logger.Info("server run !")
	s.accept(s.l)
}

func (s *Server) WithRegister(register *ServiceRegister) {
	s.register = register
}

func (s *Server) accept(lis net.Listener) {
	if s.register == nil {
		//log.Println("register is nil")
		s.logger.Error("register is nil")
		//return
	} else {
		go s.register.ListenLeaseRespChan()
	}
	//defer s.register.Close()
	for {
		conn, err := lis.Accept()
		if err != nil {
			//log.Fatal("accept error:", err)
			s.logger.Fatal("accept error", zap.Error(err))
			return
		}

		s.logger.Info("start serve a conn", zap.String("RemoteAddr", conn.RemoteAddr().String()))
		go s.serveConn(conn)
	}
}

// serveConn 处理连接
/*
	1. 先检查请求头，确认请求头是否合法，不合法则关闭连接
	2. 根据请求头中的协议类型，处理连接
*/
func (s *Server) serveConn(conn io.ReadWriteCloser) {

	defer func() {
		_ = conn.Close()
	}()
	var opt option.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		//log.Println("decode myRPC error:", err)
		s.logger.Error("decode myRPC error", zap.Error(err))
		return
	}
	s.logger.Info("receive option success", zap.Any("option", opt))

	if opt.MagicNumber != option.DefaultOption.MagicNumber {
		//log.Printf("invalid magic number %x", opt.MagicNumber)
		s.logger.Error("invalid magic number", zap.Any("opt.MagicNumber", opt.MagicNumber))
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		//log.Printf("invalid codec type %s", opt.CodecType)
		s.logger.Error("invalid codec type", zap.Any("opt.CodecType", opt.CodecType))
		return
	}
	s.serveCodec(f(conn), &opt)
}

var invalidRequest = struct{}{}

func (s *Server) serveCodec(cc codec.Codec, opt *option.Option) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	reqChan := make(chan *request, 1)
	errChan := make(chan error, 1)
	//req := &request{}
	//var err error
	go func() {
		h, err := s.readRequestHeader(cc)
		if err != nil {
			//return nil, err
			errChan <- err
			return
		}
		req := &request{
			h: h,
		}
		req.svc, req.mtype, err = s.findService(h.ServiceMethod)
		if err != nil {
			//return nil, err
			errChan <- err
			return
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
			//return nil, fmt.Errorf("argument type must be a pointer to a struct implementing proto.Message")
			errChan <- fmt.Errorf("argument type must be a pointer to a struct implementing proto.Message")
			return
		}
		argvi := req.argv.Interface()
		if err = cc.ReadBody(argvi, h.BodySize); err != nil {
			//log.Println("read body error:", err)
			s.logger.Error("read body error:", zap.Error(err))
			//return nil, err
			errChan <- err
			return
		}
		//return req, nil
		reqChan <- req
		return
	}()
	select {
	case <-time.After(time.Second * 10):
		return nil, fmt.Errorf("read Message timeout")
	case req := <-reqChan:
		return req, nil
	case err := <-errChan:
		return nil, err
	}
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
	svci, ok := s.ServiceMap.Load(serviceName)
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
		//log.Println("rpc server: write response error:", err)
		s.logger.Error("rpc server: write response error:", zap.Error(err))
	}
}

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{}, 1)
	sent := make(chan struct{}, 1)
	//s.logger.Info("start handleRequest")
	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
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
		s.sendResponse(cc, req.h, invalidRequest, sending)
	}
}

func (s *Server) Register(config RegisterConfig, rcvr interface{}) (err error) {
	// 可能不注册服务，单纯的注册到注册中心
	if rcvr != nil {
		err = s._register(rcvr)
		if err != nil {
			return err
		}
	}

	config.Host = s.Host
	s.register, err = NewServiceRegister(config)
	if err != nil {
		return err
	}
	s.register.logger = s.logger
	return
}

func (s *Server) _register(rcvr interface{}) error {
	service := newService(rcvr)
	if _, dup := s.ServiceMap.LoadOrStore(service.name, service); dup {
		return errors.New("rpc: service already defined: " + service.name)
	}
	s.logger.Info("register service success", zap.Any("service", service.name))
	return nil
}

//func Register(rcvr interface{}) error {
//	return defaultServer._register(rcvr)
//}

func (s *Server) HandleHTTP() {
	http.Handle(option.DefaultRPCPath, s)
	http.Handle(option.DefaultDebugPath, debugHTTP{s})
	//log.Println("rpc server debug path:", option.DefaultDebugPath)
	s.logger.Info("rpc server debug path", zap.Any("option.DefaultDebugPath", option.DefaultDebugPath))
}

// ServeHTTP implements an http.Handler that answers RPC requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		//log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		s.logger.Error("rpc hijacking "+req.RemoteAddr, zap.Error(err))
		return
	}
	_, _ = io.WriteString(conn, "HTTP/2.0 "+option.Connected+"\n\n")
	s.serveConn(conn)
	s.logger.Info("serve a HTTP conn")
}
