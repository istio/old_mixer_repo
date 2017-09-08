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

// THIS FILE IS AUTOMATICALLY GENERATED.

package istio_mixer_adapter_quota

import (
	"istio.io/mixer/pkg/adapter"
)

// Fully qualified name of this template
const TemplateName = "istio.mixer.adapter.quota.Quota"

// Instance is constructed by Mixer for the 'istio.mixer.adapter.quota.Quota' template.
//
// template ...
type Instance struct {
	// Name of the instance as specified in configuration.
	Name string

	// dimensions are ...
	Dimensions map[string]interface{}
}

// QuotaHandlerBuilder must be implemented by adapters if they want to
// process data associated with the Quota template.
//
// Mixer uses this interface to call into the adapter at configuration time to configure
// it with adapter-specific configuration as well as all inferred types the adapter is expected
// to handle.
type QuotaHandlerBuilder interface {
	adapter.HandlerBuilder

	// ConfigureQuotaHandler is invoked by Mixer to pass all possible Types for instances that an adapter
	// may receive at runtime. Each type holds information about the shape of the instances.
	ConfigureQuotaHandler(map[string]*Type /*Instance name -> Type*/) error
}

// QuotaHandler must be implemented by adapter code if it wants to
// process data associated with the Quota template.
//
// Mixer uses this interface to call into the adapter at request time in order to dispatch
// created instances to the adapter. Adapters take the incoming instances and do what they
// need to achieve their primary function.
//
// The name of each instance can be used as a key into the Type map supplied to the adapter
// at configuration time. These types provide descriptions of each specific instances.
type QuotaHandler interface {
	adapter.Handler

	// HandleQuota is called by Mixer at request time to deliver instances to
	// to an adapter.
	HandleQuota(*Instance, adapter.QuotaArgs) (adapter.QuotaResultLegacy, adapter.CacheabilityInfo, error)
}
