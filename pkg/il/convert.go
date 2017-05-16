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

package il

import "math"

func integerToOpcode(i int64) (uint32, uint32) {
	return uint32(i & 0xFFFFFFFF), uint32(i >> 32)
}

func doubleToOpcode(d float64) (uint32, uint32) {
	u64 := math.Float64bits(d)
	return uint32(u64 & 0xFFFFFFFF), uint32(u64 >> 32)
}
