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

package cmd

import (
	"github.com/spf13/cobra"

	"istio.io/mixer/pkg/version"
)

func versionCmd(outf outFn) *cobra.Command {
	versionCmd := cobra.Command{
		Use:   "version",
		Short: "Prints out build version information for Mixer",
		Run: func(cmd *cobra.Command, args []string) {
			outf("%s\n", version.Info)
		},
	}
	return &versionCmd
}
