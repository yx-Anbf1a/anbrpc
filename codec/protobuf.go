package codec

import (
	"bufio"
	"encoding/binary"
	"errors"
	"google.golang.org/protobuf/proto"
	"io"
)

// ProtocolCodec Protobuf 编解码器
type ProtocCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
}

//var _ Codec = (*ProtocCodec)(nil)

func NewProtoCodec(conn io.ReadWriteCloser) Codec {

	buf := bufio.NewWriter(conn) // 为conn新增一个buf缓冲区
	// 返回的是一个接口，所以需要返回一个指针
	return &ProtocCodec{conn: conn, buf: buf}
}

func (c *ProtocCodec) ReadHeader(header *Header) error {
	if header == nil {
		return errors.New("nil header")
		//return nil
	}
	// 反序列化
	headerLenBuf := make([]byte, 4)
	//_, _ = c.conn.Read(headerLenBuf)
	_, _ = io.ReadFull(c.conn, headerLenBuf)

	headerLen := binary.BigEndian.Uint32(headerLenBuf)
	//fmt.Printf("read headerLen: %d\n", headerLen)

	headerBytes := make([]byte, headerLen)
	//_, _ = c.conn.Read(headerBytes)
	_, _ = io.ReadFull(c.conn, headerBytes)

	err := proto.Unmarshal(headerBytes, header)
	return err
}

func (c *ProtocCodec) ReadBody(body interface{}, n int32) error {
	// 反序列化
	if body == nil || n == 0 {
		return errors.New("nil body")
		//return nil
	}
	buf := make([]byte, n)
	_, _ = io.ReadFull(c.conn, buf)
	//_, _ = c.conn.Read(buf)
	err := proto.Unmarshal(buf, body.(proto.Message))
	//fmt.Println("read body:", body)
	return err
}

func (c *ProtocCodec) Write0(header *Header, body interface{}) (err error) {
	defer func() {
		// 由于编码器创建在缓冲区中， 所以需要刷新缓冲区
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	_body, _ := proto.Marshal(body.(proto.Message))
	header.BodySize = int32(len(_body))

	_header, _ := proto.Marshal(header)
	_, _ = c.buf.Write(_header)
	//_ = c.buf.Flush()
	_, _ = c.buf.Write(_body)
	return
}

// 编码：头部长度(4B) + 序列化Header + Body
func (c *ProtocCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		// 由于编码器创建在缓冲区中， 所以需要刷新缓冲区
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	// 序列化Header
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = proto.Marshal(body.(proto.Message))
	} else {
		bodyBytes = make([]byte, 0)
	}
	header.BodySize = int32(len(bodyBytes))
	//fmt.Println("header.BodySize:", header.BodySize)
	headerBytes, _ := proto.Marshal(header)

	// 构造完整消息：[4B头部长度][HeaderBytes][Body]
	// 写入头部长度（网络字节序）
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, uint32(len(headerBytes)))
	if _, err = c.buf.Write(lenBytes); err != nil {
		return err
	}
	// 写入Header
	if _, err = c.buf.Write(headerBytes); err != nil {
		return err
	}
	// 写入消息体（由Header.BodySize保证一致性）
	if _, err = c.buf.Write(bodyBytes); err != nil {
		return err
	}
	return nil
}

// 解码：从Reader中解析完整消息（支持半包）
func (c *ProtocCodec) Decode(header *Header, body interface{}) (err error) {
	//io.ReadFull()
	// 1. 读取头部长度（4B）
	headerLenBuf := make([]byte, 4)
	if _, err = c.conn.Read(headerLenBuf); err != nil {
		return err
	}
	headerLen := binary.BigEndian.Uint32(headerLenBuf)

	// 2. 读取序列化Header
	headerBytes := make([]byte, headerLen)
	if _, err = c.conn.Read(headerBytes); err != nil {
		return err
	}
	//header = new(Header)
	if err = proto.Unmarshal(headerBytes, header); err != nil {
		return err
	}

	// 3. 读取消息体（根据Header.BodySize）
	bodyBytes := make([]byte, header.BodySize)
	if _, err = c.conn.Read(bodyBytes); err != nil {
		return err
	}
	if err = proto.Unmarshal(bodyBytes, body.(proto.Message)); err != nil {
		return err
	}

	return nil
}

func (c *ProtocCodec) Close() error {
	return c.conn.Close()
}

// 解码：从Reader中解析完整消息（支持半包）
