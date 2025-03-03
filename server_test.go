package main

import (
	"fmt"
	"reflect"
	"testing"
)

type Foo int

type Args struct {
	Num1, Num2 int
}

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
	var foo Foo
	s := newService(&foo)
	mtype := s.method["Add"]
	if mtype == nil {
		t.Fatal("can't find method Add")
	}
}

func TestMethodType_Calls(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Add"]

	argv := mType.newArgs()
	replyv := mType.newReply()
	argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 2}))
	err := s.call(mType, argv, replyv)

	_assert(err == nil && *replyv.Interface().(*int) == 3 && mType.NumsCalls() == 1, "failed to call Foo.Sum")
}
