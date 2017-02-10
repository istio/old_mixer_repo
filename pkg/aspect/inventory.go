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

package aspect

// APIMethod constants are used to refer to the methods handled by api.Handler
type APIMethod int

// Supported API methods
const (
	CheckMethod APIMethod = iota
	ReportMethod
	QuotaMethod
)

// Kind of aspect
type Kind int

// Supported kinds of aspects
const (
	AccessLogsKind Kind = iota
	ApplicationLogsKind
	DenialsKind
	ListsKind
	MetricsKind
	QuotasKind
)

// Name of all supported aspect kinds.
const (
	AccessLogsKindName      = "access-logs"
	ApplicationLogsKindName = "application-logs"
	DenialsKindName         = "denials"
	ListsKindName           = "lists"
	MetricsKindName         = "metrics"
	QuotasKindName          = "quotas"
)

// KindNames maps from kinds to their names.
var KindNames = map[Kind]string{
	AccessLogsKind:      AccessLogsKindName,
	ApplicationLogsKind: ApplicationLogsKindName,
	DenialsKind:         DenialsKindName,
	ListsKind:           ListsKindName,
	MetricsKind:         MetricsKindName,
	QuotasKind:          QuotasKindName,
}

// NamedKinds maps from kind names to kind enum.
var NamedKinds = map[string]Kind{
	AccessLogsKindName:      AccessLogsKind,
	ApplicationLogsKindName: ApplicationLogsKind,
	DenialsKindName:         DenialsKind,
	ListsKindName:           ListsKind,
	MetricsKindName:         MetricsKind,
	QuotasKindName:          QuotasKind,
}

// ManagerInventory holds a set of aspect managers.
type ManagerInventory map[APIMethod][]Manager

// Inventory is the authoritative set of aspect managers used by the mixer.
var Inventory = ManagerInventory{
	CheckMethod: {
		NewDenialsManager(),
		NewListsManager(),
		NewQuotasManager(),
	},

	ReportMethod: {
		NewApplicationLogsManager(),
		NewAccessLogsManager(),
	},

	QuotaMethod: {},
}
