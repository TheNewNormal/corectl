// Copyright (c) 2016 by António Meireles  <antonio.meireles@reformi.st>.
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
	"path/filepath"

	"github.com/genevera/corectl/components/common/assets"
	"github.com/genevera/corectl/components/host/session"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	utilsCmd = &cobra.Command{
		Use:   "utils",
		Short: "Some developer focused utilies",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.UsageFunc()(cmd)
		},
		Hidden: true,
	}
	manCmd = &cobra.Command{
		Use:     "genManPages",
		Short:   "Generates man pages",
		PreRunE: defaultPreRunE,
		Run: func(cmd *cobra.Command, args []string) {
			header := &doc.GenManHeader{
				Title: session.AppName(), Source: " ",
			}
			doc.GenManTree(rootCmd, header,
				filepath.Join(session.ExecutableFolder(),
					"../documentation/man/"))
		},
	}
	mkdownCmd = &cobra.Command{
		Use:     "genMarkdownDocs",
		Short:   "Generates Markdown documentation",
		PreRunE: defaultPreRunE,
		Run: func(cmd *cobra.Command, args []string) {
			doc.GenMarkdownTree(rootCmd,
				filepath.Join(session.ExecutableFolder(),
					"../documentation/markdown/"))
		},
	}
)

func init() {
	rootCmd.SetUsageTemplate(assets.Contents("cli/helpTemplate.tmpl"))
	rootCmd.PersistentFlags().BoolP("debug", "d", false,
		"adds additional verbosity, and options, directed at debugging "+
			"purposes and power users")
	utilsCmd.AddCommand(manCmd, mkdownCmd)
	rootCmd.AddCommand(utilsCmd)
}
