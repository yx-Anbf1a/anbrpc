package option

import (
	"errors"
	"myRPC/codec"
	"time"
)

const (
	Connected        = "200 Connected to Gee RPC"
	DefaultRPCPath   = "/_geeprc_"
	DefaultDebugPath = "/debug/geerpc"
)

type Option struct {
	MagicNumber    int           // 标记rpc
	CodecType      codec.Type    // 编码类型
	ConnectTimeOut time.Duration // 连接超时时间
	HandleTimeOut  time.Duration // 处理超时时间
}

var DefaultOption = &Option{
	MagicNumber:    0x3bef13,
	CodecType:      codec.ProtoTyp,
	ConnectTimeOut: time.Second * 10,
	HandleTimeOut:  time.Second * 10,
}

func ParseOption(opts ...*Option) (*Option, error) {
	// 没有设置就用默认设置
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}
