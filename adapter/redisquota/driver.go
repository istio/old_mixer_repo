// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package redisquota

import (
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
)

//TODO: Make sure the code here is thread-safe.
// ConnPool stores the info for redis connection pool.
type connPool struct {
	// TODO: add number of connections here
	pool *pool.Pool
}

type connection struct {
	client  *redis.Client
	pending uint
}

type response struct {
	response *redis.Resp
}

// Get is to get a connection from the connection pool.
func (cp *connPool) get() (*connection, error) {
	if cp == nil {
		// TODO: remove err when mock redis is added
		var err error
		return &connection{nil, 0}, err
	}
	client, err := cp.pool.Get()

	return &connection{client, 0}, err
}

// Put is to put a connection c back to the pool.
func (cp *connPool) put(c *connection) {
	// TODO: radix does not appear to track if we attempt to put a connection back with pipelined
	// responses that have not been flushed. If we are in this state, just kill the connection
	// and don't put it back in the pool.
	if cp == nil {
		// TODO: remove err when mock redis is added
		return
	}
	cp.pool.Put(c.client)
}

// NewConnPool creates a new connection to redis in the pool.
func newConnPool(redisURL string, redisSocketType string, redisPoolSize int64) (*connPool, error) {
	pool, err := pool.New(redisSocketType, redisURL, int(redisPoolSize))
	if err != nil {
		return nil, err
	}
	return &connPool{pool}, err
}

func (cp *connPool) empty() {
	// clean up all the connections in the pool.
	if cp == nil {
		// TODO: remove err when mock redis is added
		return
	}
	cp.pool.Empty()
}

func (c *connection) pipeAppend(cmd string, args ...interface{}) {
	if c != nil {
		c.client.PipeAppend(cmd, args...)
	}
	c.pending++
}

func (c *connection) pipeResponse() (*response, error) {
	if c != nil {
		c.pending--
		resp := c.client.PipeResp()
		return &response{resp}, resp.Err
	}
	// TODO: remove err when mock redis is added
	var err error
	return &response{nil}, err
}

func (r *response) int() int64 {
	i, err := r.response.Int64()

	if err != nil {
		return 0
	}
	return i
}
