package codec

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
)

// // json 编解码器
type JsonCodec struct {
	conn io.ReadWriteCloser // conn
	buf  *bufio.Writer      // 缓冲区
	dec  *json.Decoder
	enc  *json.Encoder
}

var _ Codec = (*JsonCodec)(nil)

func NewJsonCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(buf)
	return &JsonCodec{
		conn: conn,
		buf:  buf,
		dec:  dec,
		enc:  enc,
	}
}

func (c *JsonCodec) ReadHeader(header *Header) error {
	return c.dec.Decode(header)
}

func (c *JsonCodec) ReadBody(body interface{}, n int32) error {
	return c.dec.Decode(body)
}

func (c *JsonCodec) Write(header *Header, body interface{}) (err error) {
	// 将缓存刷入conn
	defer func() {
		c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	if err = c.enc.Encode(header); err != nil {
		log.Printf("rpc codec: write header error: %v\n", err)
		return
	}
	if err = c.enc.Encode(body); err != nil {
		log.Printf("rpc codec: write body error: %v\n", err)
		return
	}
	return
}
func (c *JsonCodec) Close() error {
	return c.conn.Close()
}
