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

// ConnPool interface for a redis connection pool.
type ConnPool interface {
	// Get a connection from the pool. Call Put() on the connection when done.
	Get() (Connection, error)

	// Put a connection back into the pool.
	Put(c Connection)
}

// Connection interface for a redis connection.
type Connection interface {
	// Append a command onto the pipeline queue.
	PipeAppend(command string, args ...interface{})

	// Execute the pipeline queue and wait for a response.
	PipeResponse() (response, error)
}

// Interface for a redis response.
type response interface {
	Int() int64
}
