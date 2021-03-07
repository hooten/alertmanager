// Copyright 2020 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/hashicorp/memberlist"
	"github.com/pkg/errors"

	"github.com/prometheus/alertmanager/cluster/clusterpb"
)

const (
	version      = "v1.0.0"
	uint32length = 4
)

// tlsConn wraps net.Conn with connection pooling data.
type tlsConn struct {
	connection net.Conn
	timeout    time.Duration
	lock       sync.Mutex
	live       bool
}

func dialTLSConn(addr string, timeout time.Duration, tlsConfig *tls.Config) (*tlsConn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, network, addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return &tlsConn{
		connection: conn,
		timeout:    timeout,
		live:       true,
	}, nil
}

func rcvTLSConn(conn net.Conn) *tlsConn {
	return &tlsConn{
		connection: conn,
		live:       true,
	}
}

// Write writes a byte array into the connection. It returns the number of bytes written and an error.
func (conn *tlsConn) Write(b []byte) (int, error) {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	n, err := conn.connection.Write(b)

	if err != nil {
		conn.live = false
	}
	return n, err
}

func (conn *tlsConn) alive() bool {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	return conn.live
}

// writePacket writes all the bytes in one operation so no concurrent write happens in between.
// It prefixes the message length.
func (conn *tlsConn) writePacket(fromAddr string, b []byte) error {
	msg, err := proto.Marshal(&clusterpb.MemberlistMessage{
		Version:  version,
		Kind:     clusterpb.MemberlistMessage_PACKET,
		FromAddr: fromAddr,
		Msg:      b,
	})
	if err != nil {
		return errors.Wrap(err, "unable to marshal memeberlist packet message")
	}
	buf := make([]byte, uint32length)
	binary.LittleEndian.PutUint32(buf, uint32(len(msg)))
	_, err = conn.Write(append(buf, msg...))
	return err
}

// writeStream simply signals that this is a stream connection by sending the connection type.
func (conn *tlsConn) writeStream() error {
	msg, err := proto.Marshal(&clusterpb.MemberlistMessage{
		Version: version,
		Kind:    clusterpb.MemberlistMessage_STREAM,
	})
	if err != nil {
		return errors.Wrap(err, "unable to marshal memeberlist stream message")
	}
	buf := make([]byte, uint32length)
	binary.LittleEndian.PutUint32(buf, uint32(len(msg)))
	_, err = conn.Write(append(buf, msg...))
	return err
}

// read returns a packet for packet connections or an error if there is one.
// It returns nothing if the connection is meant to be streamed.
func (conn *tlsConn) read() (*memberlist.Packet, error) {
	if conn.connection == nil {
		return nil, errors.New("nil connection")
	}
	reader := bufio.NewReader(conn.connection)
	lenBuf := make([]byte, uint32length)
	_, err := io.ReadFull(reader, lenBuf)
	if err != nil {
		return nil, errors.Wrap(err, "error reading message length")
	}
	msgLen := binary.LittleEndian.Uint32(lenBuf)
	msgBuf := make([]byte, msgLen)
	_, err = io.ReadFull(reader, msgBuf)
	if err != nil {
		return nil, errors.Wrap(err, "error reading message")
	}
	pb := clusterpb.MemberlistMessage{}
	err = proto.Unmarshal(msgBuf, &pb)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing message")
	}
	switch pb.Kind {
	case clusterpb.MemberlistMessage_STREAM:
		return nil, nil
	case clusterpb.MemberlistMessage_PACKET:
		return toPacket(pb)
	default:
		return nil, errors.New("could not read from either stream or packet channel")
	}
}

func toPacket(pb clusterpb.MemberlistMessage) (*memberlist.Packet, error) {
	addr, err := net.ResolveTCPAddr(network, pb.FromAddr)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing packet sender address")
	}
	return &memberlist.Packet{
		Buf:       pb.Msg,
		From:      addr,
		Timestamp: time.Now(),
	}, nil
}

func (conn *tlsConn) Close() error {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	conn.live = false
	if conn.connection == nil {
		return nil
	}
	return conn.connection.Close()
}
