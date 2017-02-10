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

package redisQuota

import (
	"github.com/mediocregopher/radix.v2/pool"
	"github.com/mediocregopher/radix.v2/redis"
)

type connPoolImpl struct {
	// TODO: add number of connections here
	pool *pool.Pool
}

type connectionImpl struct {
	client  *redis.Client
	pending uint
}

type responseImpl struct {
	response *redis.Resp
}

func (cp *connPoolImpl) Get() (connection, error) {
	client, err := cp.pool.Get()

	return &connectionImpl{client, 0}, err
}

func (cp *connPoolImpl) Put(c connection) {
	impl := c.(*connectionImpl)

	if impl.pending == 0 {
		cp.pool.Put(impl.client)
	} else {
		// radix does not appear to track if we attempt to put a connection back with pipelined
		// responses that have not been flushed. If we are in this state, just kill the connection
		// and don't put it back in the pool.
		impl.client.Close()
	}
}

func NewconnPoolImpl(redisUrl string, redisSocketType string, redisPoolSize int64) (connPool, error) {
	// set env variable REDIS_SOCKET_TYPE=tcp, REDIS_URL=localhost:6379
	// default pool size 10
	pool, err := pool.New(redisSocketType, redisUrl, int(redisPoolSize))
	return &connPoolImpl{pool}, err
}

func (ci *connectionImpl) PipeAppend(cmd string, args ...interface{}) {
	ci.client.PipeAppend(cmd, args...)
	ci.pending++
}

func (ci *connectionImpl) PipeResponse() (response, error) {
	// assert.Assert(this.pending > 0)
	ci.pending--

	resp := ci.client.PipeResp()
	return &responseImpl{resp}, resp.Err
}

func (ri *responseImpl) Int() int64 {
	i, _ := ri.response.Int64()

	return i
}
