package server

import (
	"bytes"

	"github.com/micro/go-micro/codec"
	"github.com/micro/go-micro/codec/jsonrpc"
	"github.com/micro/go-micro/codec/protorpc"
	"github.com/micro/go-micro/transport"
)

type rpcPlusCodec struct {
	socket transport.Socket
	codec  codec.Codec

	req *transport.Message
	buf *readWriteCloser
}

type readWriteCloser struct {
	wbuf *bytes.Buffer
	rbuf *bytes.Buffer
}

var (
	defaultCodecs = map[string]codec.NewCodec{
		"application/json":         jsonrpc.NewCodec,
		"application/json-rpc":     jsonrpc.NewCodec,
		"application/protobuf":     protorpc.NewCodec,
		"application/proto-rpc":    protorpc.NewCodec,
		"application/octet-stream": protorpc.NewCodec,
	}
)

func (rwc *readWriteCloser) Read(p []byte) (n int, err error) {
	return rwc.rbuf.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (n int, err error) {
	return rwc.wbuf.Write(p)
}

func (rwc *readWriteCloser) Close() error {
	rwc.rbuf.Reset()
	rwc.wbuf.Reset()
	return nil
}

func newRpcPlusCodec(req *transport.Message, socket transport.Socket, c codec.NewCodec) serverCodec {
	rwc := &readWriteCloser{
		rbuf: bytes.NewBuffer(req.Body),
		wbuf: bytes.NewBuffer(nil),
	}
	r := &rpcPlusCodec{
		buf:    rwc,
		codec:  c(rwc),
		req:    req,
		socket: socket,
	}
	return r
}

func (c *rpcPlusCodec) ReadRequestHeader(r *request) error {
	var m codec.Message
	err := c.codec.ReadHeader(&m, codec.Request)
	r.ServiceMethod = m.Method
	r.Seq = m.Id
	return err
}

func (c *rpcPlusCodec) ReadRequestBody(b interface{}) error {
	return c.codec.ReadBody(b)
}

func (c *rpcPlusCodec) WriteResponse(r *response, body interface{}, last bool) error {
	c.buf.wbuf.Reset()
	m := &codec.Message{
		Method: r.ServiceMethod,
		Id:     r.Seq,
		Error:  r.Error,
		Type:   codec.Response,
	}
	if err := c.codec.Write(m, body); err != nil {
		return err
	}
	return c.socket.Send(&transport.Message{
		Header: map[string]string{"Content-Type": c.req.Header["Content-Type"]},
		Body:   c.buf.wbuf.Bytes(),
	})
}

func (c *rpcPlusCodec) Close() error {
	c.buf.Close()
	c.codec.Close()
	return c.socket.Close()
}