package main

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type MethodType struct {
	method    reflect.Method // 方法本身
	ArgType   reflect.Type   // args
	ReplyType reflect.Type   // rpy
	numsCalls uint64         // 调用次数
}

// NumsCalls 获取调用次数
func (m *MethodType) NumsCalls() uint64 {
	return atomic.LoadUint64(&m.numsCalls)
}

func (m *MethodType) newArgs() reflect.Value {
	var args reflect.Value
	// 指针类型
	if m.ArgType.Kind() == reflect.Ptr {
		// 指针类型要通过.Elem()获取指针指向的值
		args = reflect.New(m.ArgType.Elem())
	} else {
		// value.Elem()指向实际的值
		args = reflect.New(m.ArgType).Elem()
	}
	return args
}

func (m *MethodType) newReply() reflect.Value {
	// reply必须是个指针
	reply := reflect.New(m.ReplyType.Elem())
	// 指针通过Elem.Kind()拿到指向元素的类型
	switch m.ReplyType.Elem().Kind() {
	case reflect.Slice:
		// 将指针指向的值设置为空 Slice
		reply.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	case reflect.Map:
		reply.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	}
	return reply

	//var reply reflect.Value
	//switch m.ReplyType.Elem().Kind() {
	//case reflect.Map:
	//	reply = reflect.MakeMap(m.ReplyType.Elem())
	//case reflect.Slice:
	//	reply = reflect.MakeSlice(m.ReplyType.Elem(), 0, 0)
	//}
	//return reply
}

type Service struct {
	name   string        // 服务名
	typ    reflect.Type  // 服务类型
	rcvr   reflect.Value // 结构体实例
	method map[string]*MethodType
}

// 传入结构体指针
func newService(rcvr interface{}) *Service {
	s := new(Service)
	s.typ = reflect.TypeOf(rcvr)                    // 指针指向的类型
	s.rcvr = reflect.ValueOf(rcvr)                  // 指针指向的值
	s.name = reflect.Indirect(s.rcvr).Type().Name() // 指针指向的对象的类型名
	// 结构体首字母必须大写
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	// 注册方法
	s.registerMethods()
	return s
}

func (s *Service) registerMethods() {
	s.method = make(map[string]*MethodType)
	// 遍历方法
	for i := 0; i < s.typ.NumMethod(); i++ {
		m := s.typ.Method(i)
		mTyp := m.Type
		if mTyp.NumIn() != 3 || mTyp.NumOut() != 1 {
			// 方法必须是3个参数和1个返回值
			continue
		}
		// error类型指针指向的值
		if mTyp.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			// 返回值必须是error
			continue
		}
		argType, replyType := mTyp.In(1), mTyp.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			// 参数必须是导出类型或者内置类型
			continue
		}
		s.method[m.Name] = &MethodType{
			method:    m,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, m.Name)
	}
}

func (s *Service) call(m *MethodType, args, reply reflect.Value) error {
	atomic.AddUint64(&m.numsCalls, 1) // 原子性
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.rcvr, args, reply})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}
