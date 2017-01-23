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

package memQuota

import (
	"testing"
)

func TestRollingWindow(t *testing.T) {
	cases := []struct {
		amount int64
		tick   int32
		avail  int64
		result bool
	}{
		{6, 1, 5, false},
		{1, 1, 4, true},
		{2, 1, 2, true},
		{2, 1, 0, true},
		{1, 1, 0, false},
		{2, 1, 0, false},
		{1, 2, 0, false},
		{1, 3, 0, false},
		{1, 4, 4, true},
		{4, 5, 0, true},
		{1, 5, 0, false},
		{1, 7, 0, true},
		{1, 7, 0, false},
		{1, 8, 3, true},
		{5, 11, 0, true},
	}

	w := newRollingWindow(5, 3)

	for i, c := range cases {
		if ok := w.alloc(c.amount, c.tick); ok != c.result {
			t.Errorf("Expecting %v got %v, case %d", c.result, ok, i)
		}

		if w.available() != c.avail {
			t.Errorf("Expecting %d available, got %d, case %d", c.avail, w.available(), i)
		}
	}
}
