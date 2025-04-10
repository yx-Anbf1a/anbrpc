package test_service

import (
	"google.golang.org/protobuf/proto"
	"time"
)

var _ proto.Message = (*FBoo)(nil)
var _ proto.Message = (*FBooArgs)(nil)
var _ proto.Message = (*FBooReply)(nil)

func (fb *FBoo) Sum(args *FBooArgs) (reply *FBooReply) {
	reply = new(FBooReply)
	reply.Num = args.Num1 + args.Num2
	return
}
func (fb *FBoo) Sleep(args *FBooArgs) (reply *FBooReply) {
	time.Sleep(time.Second * time.Duration(args.Num1))
	reply = new(FBooReply)
	reply.Num = args.Num1 + args.Num2
	return
}
