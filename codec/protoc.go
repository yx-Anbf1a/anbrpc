package codec

import (
	"bufio"
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

//func (c *ProtocCodec) ParseMessage(message *Message) error {
//	buf := make([]byte, 1024)
//	n, _ := c.conn.Read(buf)
//	proto.Unmarshal(buf[:n], message)
//	return nil
//}

func (c *ProtocCodec) ReadHeader(header *Header) error {
	// 反序列化
	b := make([]byte, 100)
	n, _ := c.conn.Read(b)
	newBuf := make([]byte, n)
	copy(newBuf, b[:n])
	err := proto.Unmarshal(newBuf, header)
	//fmt.Println("read header:", header)
	return err
}

func (c *ProtocCodec) ReadBody(body interface{}) error {
	// 反序列化
	if body == nil {
		return nil
	}
	buf := make([]byte, 1024)
	n, _ := c.conn.Read(buf)
	proto.Unmarshal(buf[:n], body.(proto.Message))
	//fmt.Println("read body:", body)
	return nil
}

func (c *ProtocCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		// 由于编码器创建在缓冲区中， 所以需要刷新缓冲区
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	b, _ := proto.Marshal(header)
	//fmt.Println("write header:", b)
	c.buf.Write(b)
	c.buf.Flush()
	d, _ := proto.Marshal(body.(proto.Message))
	//fmt.Println("write body:", d)
	c.buf.Write(d)
	return
}

func (c *ProtocCodec) Close() error {
	return c.conn.Close()
}
