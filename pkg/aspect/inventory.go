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

// APIBinding associates an aspect with an API method
type APIBinding struct {
	Aspect Manager
	Method APIMethod
}

// Kind of aspect
type Kind int

// Supports kinds of aspects
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

// Inventory returns a manager inventory that contains
// all available aspect managers
func Inventory() []APIBinding {
	// Update the following list to add a new Aspect manager
	return []APIBinding{
		{NewDenialsManager(), CheckMethod},
		{NewListsManager(), CheckMethod},
		{NewApplicationLogsManager(), ReportMethod},
		{NewAccessLogsManager(), ReportMethod},
		{NewQuotasManager(), CheckMethod},
		{NewQuotasManager(), QuotaMethod},
	}
}
