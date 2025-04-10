package codec

import (
	"io"
)

//// 定义请求头
//type Header struct {
//	ServiceMethod string // 服务名和方法名
//	Seq           uint64 // 请求的序列号
//	Error         string // 错误信息
//}

// Codec 编解码器，能够读写请求头，请求体，及写入数据
type Codec interface {
	io.Closer // 基础io接口
	ReadHeader(header *Header) error
	ReadBody(body interface{}) error
	Write(header *Header, body interface{}) error
}
type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
	ProtoTyp Type = "proto"
)

type NewCodecFunc func(io.ReadWriteCloser) Codec

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[JsonType] = NewJsonCodec
	NewCodecFuncMap[GobType] = NewGobCodec
	NewCodecFuncMap[ProtoTyp] = NewProtoCodec
}
