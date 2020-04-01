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
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByte(t *testing.T) {
	testCases := []struct {
		input    connectionType
		expected byte
		error    bool
	}{
		{input: stream, expected: 's', error: false},
		{input: packet, expected: 'p', error: false},
		{input: 556, expected: 'n', error: true},
	}

	for _, tc := range testCases {
		b, err := tc.input.byte()
		if tc.error {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
		}
		require.Equal(t, tc.expected, b)
	}
}

func TestBytes(t *testing.T) {
	testCases := []struct {
		input    connectionType
		expected []byte
		error    bool
	}{
		{input: stream, expected: []byte{'s'}, error: false},
		{input: packet, expected: []byte{'p'}, error: false},
		{input: 3, expected: nil, error: true},
	}

	for _, tc := range testCases {
		bytes, err := tc.input.bytes()
		if tc.error {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
		}
		require.Equal(t, tc.expected, bytes)
	}
}

func TestConnType(t *testing.T) {
	testCases := []struct {
		input    byte
		expected connectionType
		error    bool
	}{
		{input: 's', expected: stream, error: false},
		{input: 'p', expected: packet, error: false},
		{input: 'n', expected: none, error: true},
	}

	for _, tc := range testCases {
		ct, err := connType(tc.input)
		if tc.error {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
		}
		require.Equal(t, tc.expected, ct)
	}
}

func TestWriteStream(t *testing.T) {
	w, r := net.Pipe()
	wrapper := &connWrapper{
		connection: w,
	}
	defer r.Close()
	go func() {
		writeStream(wrapper)
		w.Close()
	}()
	out, err := ioutil.ReadAll(r)
	require.Nil(t, err)
	expected, err := stream.bytes()
	require.Nil(t, err)
	require.Equal(t, expected, out)
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
			writePacket(&connWrapper{connection: w}, tc.fromAddr, []byte(tc.msg))
			w.Close()
		}()
		out, err := ioutil.ReadAll(r)
		require.Nil(t, err)
		addr := append([]byte(tc.fromAddr), delim)
		length := append([]byte(strconv.Itoa(len(tc.msg))), delim)
		prefix := append(append([]byte{'p'}, addr...), length...)
		expected := append(prefix, []byte(tc.msg)...)
		require.Nil(t, err)
		require.Equal(t, expected, out)
	}
}

func TestRead_Stream(t *testing.T) {
	w, r := net.Pipe()
	defer r.Close()
	go func() {
		writeStream(&connWrapper{connection: w})
		w.Close()
	}()
	packet, err := read(r)
	require.Nil(t, packet)
	require.Nil(t, err)
}

func TestRead_Packet(t *testing.T) {
	testCases := []struct {
		addr  string
		s     string
		error bool
	}{
		{addr: "127.0.0.1:9094", s: "", error: true},
		{addr: "10.0.0.1:8001", s: "test", error: false},
		{addr: "127.0.0.1:9094", s: "-1", error: false},
	}
	for _, tc := range testCases {
		w, r := net.Pipe()
		defer r.Close()
		go func() {
			writePacket(&connWrapper{connection: w}, tc.addr, []byte(tc.s))
			w.Close()
		}()
		packet, err := read(r)
		if tc.error {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
			require.Equal(t, []byte(tc.s), packet.Buf)
			require.Equal(t, tc.addr, packet.From.String())
		}
	}
}

func TestRead_Error(t *testing.T) {
	w, r := net.Pipe()
	defer r.Close()
	go func() {
		write(&connWrapper{connection: w}, none, []byte("invalid type"))
		w.Close()
	}()
	packet, err := read(r)
	require.Nil(t, packet)
	require.NotNil(t, err)
}

func TestRead_Nil(t *testing.T) {
	packet, err := read(nil)
	require.Nil(t, packet)
	require.NotNil(t, err)
}

func TestReadPacket(t *testing.T) {
	testCases := []struct {
		error bool
		addr  string
		s     string
	}{
		{addr: "127.0.0.1:9094", s: "", error: true},
		{addr: "127.0.0.1:9094", s: "string", error: false},
		{addr: ":0", s: "s", error: false},
	}
	for _, tc := range testCases {
		s := tc.addr + "\n" + strconv.Itoa(len(tc.s)) + "\n" + tc.s
		reader := bufio.NewReader(strings.NewReader(s))
		packet, err := readPacket(reader)
		if tc.error {
			require.NotNil(t, err)
			require.Nil(t, packet)
		} else {
			require.Nil(t, err)
			require.Equal(t, []byte(tc.s), packet.Buf)
			require.Equal(t, tc.addr, packet.From.String())
		}
	}
}

func TestReadPrefixPart(t *testing.T) {
	testCases := []string{
		"invalid",
		"\n",
		"0\n",
		"string\n",
	}
	for _, s := range testCases {
		reader := bufio.NewReader(strings.NewReader(s))
		part, err := readPrefixPart(reader)
		if s[len(s)-1] == '\n' {
			require.Equal(t, strings.TrimRight(s, "\n"), part)
		} else {
			require.NotNil(t, err)
		}
	}
}
