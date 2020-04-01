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
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/pkg/errors"
)

const delim byte = '\n'

type connectionType int

const (
	stream connectionType = iota
	packet
	none
)

func (ct connectionType) byte() (byte, error) {
	if ct < 0 || ct > 1 {
		return 'n', errors.New("invalid connection type")
	}
	return [...]byte{'s', 'p'}[ct], nil
}

func (ct connectionType) bytes() ([]byte, error) {
	b, err := ct.byte()
	if err != nil {
		return nil, err
	}
	return []byte{b}, nil
}

func connType(b byte) (connectionType, error) {
	switch b {
	case 's':
		return stream, nil
	case 'p':
		return packet, nil
	default:
		return none, errors.New("invalid byte")
	}
}

// writePacket writes all the bytes in one operation so no concurrent write happens in between.
// It prefixes the connection type, the from address and the message length.
func writePacket(conn net.Conn, fromAddr string, b []byte) error {
	addr := append([]byte(fromAddr), delim)
	length := append([]byte(strconv.Itoa(len(b))), delim)
	prefix := append(addr, length...)
	return write(conn, packet, append(prefix, b...))
}

// writeStream simply signals that this is a stream connection by sending the connection type.
func writeStream(conn net.Conn) error {
	return write(conn, stream, []byte{})
}

func write(conn net.Conn, ct connectionType, b []byte) error {
	prefix, err := ct.bytes()
	if err != nil {
		return errors.Wrap(err, "unable to write magic bytes")
	}
	bytes := append(prefix, b...)
	_, err = conn.Write(bytes)
	return errors.Wrap(err, "failed to write packet")
}

// read returns a packet for packet connections or an error if there is one.
// It returns nothing if the connection is meant to be streamed.
func read(conn net.Conn) (*memberlist.Packet, error) {
	if conn == nil {
		return nil, errors.New("nil connection")
	}
	reader := bufio.NewReader(conn)
	b, err := reader.ReadByte()
	if err != nil {
		return nil, errors.Wrap(err, "error reading connection type") //here EOF
	}
	ct, err := connType(b)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing connection type")
	}
	switch ct {
	case stream:
		return nil, nil
	case packet:
		return readPacket(reader)
	default:
		return nil, errors.New("could not read from either stream or packet channel")
	}
}

func readPacket(reader *bufio.Reader) (*memberlist.Packet, error) {
	addrStr, err := readPrefixPart(reader)
	if err != nil {
		return nil, errors.Wrap(err, "error reading packet sender address")
	}
	lenStr, err := readPrefixPart(reader)
	if err != nil {
		return nil, errors.Wrap(err, "error reading packet message length")
	}
	addr, err := net.ResolveTCPAddr(network, addrStr)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing packet sender address")
	}
	length, err := strconv.Atoi(lenStr)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing packet message length")
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, errors.Wrap(err, "error reading packet message")
	}
	if len(buf) < 1 {
		return nil, errors.New("packet too short")
	}

	return &memberlist.Packet{
		Buf:       buf,
		From:      addr,
		Timestamp: time.Now(),
	}, nil
}

func readPrefixPart(reader *bufio.Reader) (string, error) {
	part, err := reader.ReadString(delim)
	if err != nil {
		return "", errors.Wrap(err, "error reading packet part")
	}
	return strings.TrimRight(part, string(delim)), nil
}
