// Copyright 2017 Istio Authors
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
	"sync"
	"testing"
)

func TestPool(t *testing.T) {
	pool, _ := newConnPool("localhost:6379", "tcp", 10)
	// TODO: remove comment after mock redis is added.
	/* if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}
	*/
	var wg sync.WaitGroup
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 100; i++ {
				conn, err := pool.get()
				if err != nil {
					t.Errorf("Unable to get connection from pool: %v", err)
				}
				pool.put(conn)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	pool.empty()
}

// TODO: add mock redis for more unit tests.
/*
func TestPipe(t *testing.T) {
	pool, err := newConnPool("localhost:6379", "tcp", 10)

	if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}

	conn, err := pool.get()
	if err != nil {
		t.Errorf("Unable to create aspect: %v", err)
	}

	conn.pipeAppend("INCRBY", "key1", "result1")
	if conn.pending != 1 {
		t.Errorf("Unable to pipe command: %v", err)
	}
}
*/
