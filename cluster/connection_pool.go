// Copyright 2020 Prometheus Team
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
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

type connectionPool struct {
	pool    map[string]*connWrapper
	tlsConf *tls.Config
}

// connWrapper wraps net.Conn with connection pooling data.
type connWrapper struct {
	connection net.Conn
	timeout    time.Duration
	lock       sync.Mutex
	live       bool
}

func newConnectionPool(tlsConf *tls.Config) *connectionPool {
	return &connectionPool{
		pool:    make(map[string]*connWrapper),
		tlsConf: tlsConf,
	}
}

// borrowConnection returns a *connWrapper from the pool. It does not need to be returned because
// there is a per-connection locking mechanism.
func (pool *connectionPool) borrowConnection(addr string, timeout time.Duration) (*connWrapper, error) {
	var err error
	key := fmt.Sprintf("%s/%d", addr, int64(timeout))
	conn, ok := pool.pool[key]
	if !ok || !conn.isAlive() {
		conn, err = pool.createConnection(addr, timeout)
		if err != nil {
			return nil, err
		}
		pool.pool[key] = conn
	}
	return conn, nil
}

func (pool *connectionPool) createConnection(addr string, timeout time.Duration) (*connWrapper, error) {
	dialer := &net.Dialer{Timeout: DefaultTcpTimeout}
	conn, err := tls.DialWithDialer(dialer, network, addr, pool.tlsConf)
	if err != nil {
		return nil, err
	}

	return &connWrapper{
		connection: conn,
		timeout:    timeout,
		live:       true,
	}, nil
}

// Write writes a byte array into the connection. It returns the number of bytes written and an error.
func (c *connWrapper) Write(b []byte) (int, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	n, err := c.connection.Write(b)

	if err != nil {
		c.live = false
	}
	return n, err
}

func (c *connWrapper) isAlive() bool {
	return c.live
}
