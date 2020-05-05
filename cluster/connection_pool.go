package cluster

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// connectionPool Struct hold all connction mapping
type connectionPool struct {
	pool    map[string]*connWrapper
	tlsConf *tls.Config
}

// connWrapper struct is wrapper on net.Conn
type connWrapper struct {
	connection net.Conn
	timeout    time.Duration
	lock       sync.Mutex
	live       bool
}

// newConnectionPool return connection pool instance
func newConnectionPool(tlsConf *tls.Config) *connectionPool {
	return &connectionPool{
		pool:    make(map[string]*connWrapper),
		tlsConf: tlsConf,
	}
}

func (pool *connectionPool) borrowConnection(addr string, timeout time.Duration) (*connWrapper, error) {
	var err error
	key := fmt.Sprintf("%s/%s", addr, timeout)
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

// Write will write byte array.
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
