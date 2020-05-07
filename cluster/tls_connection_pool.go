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
	"time"
)

type connectionPool struct {
	conns     map[string]*tlsConn
	tlsConfig *tls.Config
}

func newConnectionPool(tlsConfig *tls.Config) *connectionPool {
	return &connectionPool{
		conns:     make(map[string]*tlsConn),
		tlsConfig: tlsConfig,
	}
}

// borrowConnection returns a *tlsConn from the pool. The connection does not
// need to be returned to the pool because there is per-connection locking.
func (pool *connectionPool) borrowConnection(addr string, timeout time.Duration) (*tlsConn, error) {
	var err error
	key := fmt.Sprintf("%s/%d", addr, int64(timeout))
	conn, ok := pool.conns[key]
	if !ok || !conn.alive() {
		conn, err = dialTLSConn(addr, timeout, pool.tlsConfig)
		if err != nil {
			return nil, err
		}
		pool.conns[key] = conn
	}
	return conn, nil
}
