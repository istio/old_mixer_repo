// Copyright 2017 Istio Authors.
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

// !!!!!!!!!!!!!!!!!!!!! WARNING !!!!!!!!!!!!!!!!!!!!!!!!
// THIS IS AUTO GENERATED FILE - SIMULATED - HAND WRITTEN

package adapter

import sample_report "istio.io/mixer/pkg/templates/sample/report"

// Registrar2 is used by adapters to register themselves as processors of one or more available templates.
type Registrar2 interface {
	RegisterSampleProcessor(processor sample_report.SampleProcessor)
}

// RegisterFn2 is a function Mixer invokes to trigger adapters to register
// themselves as processors of one or more templates. It must succeed or panic().
type RegisterFn2 func(Registrar2)
