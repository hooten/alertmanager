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
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteStream(t *testing.T) {
	w, r := net.Pipe()
	conn := &tlsConn{
		connection: w,
	}
	defer r.Close()
	go func() {
		conn.writeStream()
		w.Close()
	}()
	packet, err := rcvTLSConn(r).read()
	require.Nil(t, err)
	require.Nil(t, packet)
}

func TestWritePacket(t *testing.T) {
	testCases := []struct {
		fromAddr string
		msg      string
	}{
		{fromAddr: "127.0.0.1:8001", msg: ""},
		{fromAddr: "10.0.0.4:9094", msg: "hello"},
		{fromAddr: "127.0.0.1:8001", msg: "0"},
	}
	for _, tc := range testCases {
		w, r := net.Pipe()
		defer r.Close()
		go func() {
			conn := &tlsConn{connection: w}
			conn.writePacket(tc.fromAddr, []byte(tc.msg))
			w.Close()
		}()
		packet, err := rcvTLSConn(r).read()
		require.Nil(t, err)
		require.Equal(t, tc.msg, string(packet.Buf))
		require.Equal(t, tc.fromAddr, packet.From.String())

	}
}

func TestRead_Nil(t *testing.T) {
	packet, err := (&tlsConn{}).read()
	require.Nil(t, packet)
	require.NotNil(t, err)
}
