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

package runtime

import (
	"context"

	"github.com/golang/protobuf/proto"

	"istio.io/mixer/pkg/config/store"
)

// Store abstraction used here.
// This file is temporary.
// It file will be removed when config.store.Store2 interface is added.
type Store interface {
	Init(ctx context.Context, kinds map[string]proto.Message) error
	Watch(ctx context.Context) (<-chan store.Event, error)
	List() map[store.Key]proto.Message
	Get(key store.Key, spec proto.Message) error
}
