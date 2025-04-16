package server

import (
	"fmt"
	"github.com/yx-Anbf1a/anbrpc/test_service"
	"reflect"
	"testing"
)

type Foo int

//	type Args struct {
//		Num1, Num2 int
//	}
type Args struct{ Num1, Num2 int }

func (f Foo) Add(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestNewService(t *testing.T) {
	var foo test_service.FBoo
	s := newService(&foo)
	mtype := s.method["Sum"]
	if mtype == nil {
		t.Fatal("can't find method Add")
	}
}

func TestMethodType_Calls(t *testing.T) {
	var foo test_service.FBoo
	s := newService(&foo)
	mType := s.method["Sum"]

	argv := mType.newArgs()
	replyv := mType.newReply()
	argv.Elem().Set(reflect.ValueOf(test_service.FBooArgs{Num1: 1, Num2: 2}))
	err := s.call(mType, argv, replyv)
	fmt.Println(replyv.Interface().(*test_service.FBooReply).Num)
	_assert(err == nil && replyv.Interface().(*test_service.FBooReply).Num == 3 && mType.NumsCalls() == 1, "failed to call Foo.Sum")
}
