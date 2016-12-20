// Copyright 2016 Google Inc.
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

package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/grpc"

	mixerpb "istio.io/mixer/api/v1"
)

type clientState struct {
	client     mixerpb.MixerClient
	connection *grpc.ClientConn
}

func createAPIClient(port string) (*clientState, error) {
	cs := clientState{}

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	var err error
	if cs.connection, err = grpc.Dial(port, opts...); err != nil {
		return nil, err
	}

	cs.client = mixerpb.NewMixerClient(cs.connection)
	return &cs, nil
}

func deleteAPIClient(cs *clientState) {
	cs.connection.Close()
	cs.client = nil
	cs.connection = nil
}

type assignFunc func(index int32, value string) error

func decomposeAttributes(attrs string, dict map[int32]string, assign assignFunc) error {
	if len(attrs) > 0 {
		for _, a := range strings.Split(attrs, ",") {
			i := strings.Index(a, "=")
			if i < 0 {
				return fmt.Errorf("Attribute value %v does not include an = sign", a)
			} else if i == 0 {
				return fmt.Errorf("Attribute value %v does not contain a valid name", a)
			}
			name := a[0:i]
			value := a[i+1:]
			index := int32(len(dict))
			dict[index] = name

			err := assign(index, value)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func parseAttributes(rootArgs *rootArgs) (*mixerpb.Attributes, error) {
	attrs := mixerpb.Attributes{}
	attrs.Dictionary = make(map[int32]string)

	// once again, the following boilerplate would be more succinct with generics...

	if err := decomposeAttributes(rootArgs.stringAttributes, attrs.Dictionary, func(index int32, value string) error {
		if attrs.StringAttributes == nil {
			attrs.StringAttributes = make(map[int32]string)
		}
		attrs.StringAttributes[index] = value
		return nil
	}); err != nil {
		return nil, err
	}

	if err := decomposeAttributes(rootArgs.int64Attributes, attrs.Dictionary, func(index int32, value string) error {
		if attrs.Int64Attributes == nil {
			attrs.Int64Attributes = make(map[int32]int64)
		}
		var e error
		attrs.Int64Attributes[index], e = strconv.ParseInt(value, 10, 64)
		return e
	}); err != nil {
		return nil, err
	}

	if err := decomposeAttributes(rootArgs.doubleAttributes, attrs.Dictionary, func(index int32, value string) error {
		if attrs.DoubleAttributes == nil {
			attrs.DoubleAttributes = make(map[int32]float64)
		}
		var e error
		attrs.DoubleAttributes[index], e = strconv.ParseFloat(value, 64)
		return e
	}); err != nil {
		return nil, err
	}

	if err := decomposeAttributes(rootArgs.boolAttributes, attrs.Dictionary, func(index int32, value string) error {
		if attrs.BoolAttributes == nil {
			attrs.BoolAttributes = make(map[int32]bool)
		}
		var e error
		attrs.BoolAttributes[index], e = strconv.ParseBool(value)
		return e
	}); err != nil {
		return nil, err
	}

	if err := decomposeAttributes(rootArgs.timestampAttributes, attrs.Dictionary, func(index int32, value string) error {
		if attrs.TimestampAttributes == nil {
			attrs.TimestampAttributes = make(map[int32]*timestamp.Timestamp)
		}
		time, e := time.Parse(time.RFC3339, value)
		if e != nil {
			return e
		}

		var ts *timestamp.Timestamp
		ts, e = ptypes.TimestampProto(time)
		if e != nil {
			return e
		}
		attrs.TimestampAttributes[index] = ts
		return nil
	}); err != nil {
		return nil, err
	}

	if err := decomposeAttributes(rootArgs.bytesAttributes, attrs.Dictionary, func(index int32, value string) error {
		if attrs.BytesAttributes == nil {
			attrs.BytesAttributes = make(map[int32][]uint8)
		}
		var bytes []uint8
		for _, s := range strings.Split(value, ":") {
			b, e := strconv.ParseInt(s, 16, 8)
			if e != nil {
				return e
			}
			bytes = append(bytes, uint8(b))
		}
		attrs.BytesAttributes[index] = bytes
		return nil
	}); err != nil {
		return nil, err
	}

	return &attrs, nil
}

func errorf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}
