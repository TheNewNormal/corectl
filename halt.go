// Copyright 2015 - António Meireles  <antonio.meireles@reformi.st>
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
//

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	haltCmd = &cobra.Command{
		Use:     "halt",
		Short:   "halts a running CoreOS instance",
		Run:     haltCommand,
		Aliases: []string{"kill"},
	}
)

func haltCommand(cmd *cobra.Command, args []string) {
	fmt.Println("TBD")
}
func init() {
	RootCmd.AddCommand(haltCmd)
}
