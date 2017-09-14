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

package advance

import (
	"istio.io/mixer/pkg/adapter"
	spyAdapter "istio.io/mixer/test/e2e/advance/spyAdapter"
)

func constructAdapterInfos(adptBehaviors []spyAdapter.AdapterBehavior) ([]adapter.InfoFn, []*spyAdapter.Adapter) {
	var adapterInfos []adapter.InfoFn = make([]adapter.InfoFn, 0)
	spyAdapters := make([]*spyAdapter.Adapter, 0)
	for _, b := range adptBehaviors {
		sa := spyAdapter.NewSpyAdapter(b)
		spyAdapters = append(spyAdapters, sa)
		adapterInfos = append(adapterInfos, sa.GetAdptInfoFn())
	}
	return adapterInfos, spyAdapters
}
