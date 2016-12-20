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
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
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

func parseAttributes(rootArgs *rootArgs) (*mixerpb.Attributes, error) {
	attrs := mixerpb.Attributes{}
	attrs.Dictionary = make(map[int32]string)

	// once again, the following boilerplate would be more succinct with generics...

	type AttrType int
	const (
		STRING AttrType = iota
		INT64
		DOUBLE
		BOOL
		TIMESTAMP
		BYTES
	)

	for i := STRING; i <= BYTES; i++ {
		var a string
		switch i {
		case STRING:
			a = rootArgs.stringAttributes
		case INT64:
			a = rootArgs.int64Attributes
		case DOUBLE:
			a = rootArgs.doubleAttributes
		case BOOL:
			a = rootArgs.boolAttributes
		case TIMESTAMP:
			a = rootArgs.timestampAttributes
		case BYTES:
			a = rootArgs.boolAttributes
		}

		if len(a) > 0 {
			for _, a := range strings.Split(a, ",") {
				eq := strings.Index(a, "=")
				if eq < 0 {
					return nil, fmt.Errorf("Attribute value %v does not include an = sign", a)
				}
				if eq == 0 {
					return nil, fmt.Errorf("Attribute value %v does not contain a valid name", a)
				}
				name := a[0:eq]
				value := a[eq+1:]

				index := int32(len(attrs.Dictionary))
				attrs.Dictionary[index] = name

				switch i {
				case STRING:
					if attrs.StringAttributes == nil {
						attrs.StringAttributes = make(map[int32]string)
					}
					attrs.StringAttributes[index] = value

				case INT64:
					if attrs.Int64Attributes == nil {
						attrs.Int64Attributes = make(map[int32]int64)
					}
					var err error
					if attrs.Int64Attributes[index], err = strconv.ParseInt(value, 10, 64); err != nil {
						return nil, err
					}

				case DOUBLE:
					if attrs.DoubleAttributes == nil {
						attrs.DoubleAttributes = make(map[int32]float64)
					}
					var err error
					if attrs.DoubleAttributes[index], err = strconv.ParseFloat(value, 64); err != nil {
						return nil, err
					}

				case BOOL:
					if attrs.BoolAttributes == nil {
						attrs.BoolAttributes = make(map[int32]bool)
					}
					var err error
					if attrs.BoolAttributes[index], err = strconv.ParseBool(value); err != nil {
						return nil, err
					}

				case TIMESTAMP:
					if attrs.TimestampAttributes == nil {
						attrs.TimestampAttributes = make(map[int32]*timestamp.Timestamp)
					}
					time, err := time.Parse(time.RFC3339, value)
					if err != nil {
						return nil, err
					}

					var ts *timestamp.Timestamp
					if ts, err = ptypes.TimestampProto(time); err != nil {
						return nil, err
					}
					attrs.TimestampAttributes[index] = ts

				case BYTES:
					if attrs.BytesAttributes == nil {
						attrs.BytesAttributes = make(map[int32][]uint8)
					}
					var bytes []uint8
					for _, s := range strings.Split(value, ":") {
						b, err := strconv.ParseInt(s, 16, 8)
						if err != nil {
							return nil, err
						}
						bytes = append(bytes, uint8(b))
					}
					attrs.BytesAttributes[index] = bytes
				}
			}
		}
	}

	return &attrs, nil
}

func errorf(format string, a ...interface{}) {
	glog.Errorf(format+"\n", a...)
}
