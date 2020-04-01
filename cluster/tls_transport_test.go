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
	context2 "context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/alertmanager/config"
	"github.com/stretchr/testify/require"
)

var logger = log.NewNopLogger()

func TestNewTLSTransport(t *testing.T) {
	testCases := []struct {
		bindAddr    string
		bindPort    int
		tlsConfFile string
		err         string
	}{
		{bindAddr: "", err: "invalid bind address \"\""},
		{bindAddr: "abc123", err: "invalid bind address \"abc123\""},
		{bindAddr: localhost, bindPort: 0, tlsConfFile: "testdata/tls_config_node1.yml"},
		{bindAddr: localhost, bindPort: 9094, tlsConfFile: "testdata/tls_config_node2.yml"},
	}
	l := log.NewNopLogger()
	for _, tc := range testCases {
		tlsConf, _ := config.GetTLSConfig(tc.tlsConfFile)
		transport, err := NewTLSTransport(context2.Background(), l, nil, tc.bindAddr, tc.bindPort, DefaultTLSGossipHandlers, tlsConf)
		if len(tc.err) > 0 {
			require.Equal(t, tc.err, err.Error())
			require.Nil(t, transport)
		} else {
			require.Nil(t, err)
			require.Equal(t, tc.bindAddr, transport.bindAddr)
			require.Equal(t, tc.bindPort, transport.bindPort)
			require.Equal(t, l, transport.logger)
			require.Equal(t, tlsConf, transport.tlsConf)
			require.NotNil(t, transport.listener)
			transport.Shutdown()
		}
	}
}

const localhost = "127.0.0.1"

func TestFinalAdvertiseAddr(t *testing.T) {
	testCases := []struct {
		bindAddr      string
		bindPort      int
		inputIp       string
		inputPort     int
		expectedIp    string
		expectedPort  int
		expectedError string
	}{
		{bindAddr: localhost, bindPort: 9094, inputIp: "10.0.0.5", inputPort: 54231, expectedIp: "10.0.0.5", expectedPort: 54231},
		{bindAddr: localhost, bindPort: 9093, inputIp: "invalid", inputPort: 54231, expectedError: "failed to parse advertise address \"invalid\""},
		{bindAddr: "0.0.0.0", bindPort: 0, inputIp: "", inputPort: 0, expectedIp: "random"},
		{bindAddr: localhost, bindPort: 0, inputIp: "", inputPort: 0, expectedIp: localhost},
		{bindAddr: localhost, bindPort: 9095, inputIp: "", inputPort: 0, expectedIp: localhost, expectedPort: 9095},
	}
	for _, tc := range testCases {
		tlsConf, _ := config.GetTLSConfig("testdata/tls_config_node1.yml")
		transport, err := NewTLSTransport(context2.Background(), logger, nil, tc.bindAddr, tc.bindPort, DefaultTLSGossipHandlers, tlsConf)
		require.Nil(t, err)
		defer transport.Shutdown()
		ip, port, err := transport.FinalAdvertiseAddr(tc.inputIp, tc.inputPort)
		if len(tc.expectedError) > 0 {
			require.Equal(t, tc.expectedError, err.Error())
		} else {
			require.Nil(t, err)
			if tc.expectedPort == 0 {
				require.True(t, tc.expectedPort < port)
				require.True(t, uint32(port) <= uint32(1<<32 - 1))
			} else {
				require.Equal(t, tc.expectedPort, port)
			}
			if tc.expectedIp == "random" {
				require.NotNil(t, ip)
			} else {
				require.Equal(t, tc.expectedIp, ip.String())
			}
		}
	}
}

func TestWriteTo(t *testing.T) {
	tlsConf1, _ := config.GetTLSConfig("testdata/tls_config_node1.yml")
	t1, _ := NewTLSTransport(context2.Background(), logger, nil, "127.0.0.1", 0, DefaultTLSGossipHandlers, tlsConf1)
	defer t1.Shutdown()

	tlsConf2, _ := config.GetTLSConfig("testdata/tls_config_node2.yml")
	t2, _ := NewTLSTransport(context2.Background(), logger, nil, "127.0.0.1", 0, DefaultTLSGossipHandlers, tlsConf2)
	defer t2.Shutdown()

	from := fmt.Sprintf("%s:%d", t1.bindAddr, t1.GetAutoBindPort())
	to := fmt.Sprintf("%s:%d", t2.bindAddr, t2.GetAutoBindPort())
	sent := []byte(("test packet"))

	_, err := t1.WriteTo(sent, to)
	require.Nil(t, err)
	packet := <- t2.PacketCh()
	require.Equal(t, sent, packet.Buf)
	require.Equal(t, from, packet.From.String())
}

func TestDialTimout(t *testing.T) {
	tlsConf1, _ := config.GetTLSConfig("testdata/tls_config_node1.yml")
	t1, _ := NewTLSTransport(context2.Background(), logger, nil, "127.0.0.1", 0, DefaultTLSGossipHandlers, tlsConf1)
	defer t1.Shutdown()

	tlsConf2, _ := config.GetTLSConfig("testdata/tls_config_node2.yml")
	t2, _ := NewTLSTransport(context2.Background(), logger, nil, "127.0.0.1", 0, DefaultTLSGossipHandlers, tlsConf2)
	defer t2.Shutdown()

	addr := fmt.Sprintf("%s:%d", t2.bindAddr, t2.GetAutoBindPort())
	from, err := t1.DialTimeout(addr, 5 * time.Second)
	defer from.Close()
	require.Nil(t, err)
	to := <- t2.StreamCh()

	sent := []byte(("test stream"))
	_, err = from.Write(sent)
	require.Nil(t, err)

	reader := bufio.NewReader(to)
	buf := make([]byte, len(sent))
	n, err := io.ReadFull(reader, buf)
	require.Nil(t, err)
	require.Equal(t, len(sent), n)
	require.Equal(t, sent, buf)
}


type logWr struct {
	bytes []byte
}

func (l *logWr) Write(p []byte) (n int, err error) {
	l.bytes = append(l.bytes, p...)
	return len(p), nil
}

func TestShutdown(t *testing.T) {
	tlsConf1, _ := config.GetTLSConfig("testdata/tls_config_node1.yml")
	l := &logWr{}
	t1, _ := NewTLSTransport(context2.Background(), log.NewLogfmtLogger(l), nil, "127.0.0.1", 0, DefaultTLSGossipHandlers, tlsConf1)
	// Sleeping to make sure listeners have started and can subsequently be shut down gracefully.
	time.Sleep(500 * time.Millisecond)
	err := t1.Shutdown()
	require.Nil(t, err)
	require.NotContains(t, string(l.bytes), "use of closed network connection")
	require.Contains(t, string(l.bytes), "shutting down tls transport")
}
