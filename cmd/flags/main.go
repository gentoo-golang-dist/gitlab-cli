package main

import (
	"cmp"
	"encoding/json"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.com/gitlab-org/cli/commands"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
)

func main() {
	cmdFactory := cmdutils.NewFactory()
	cmdFactory.IO.StdErr = os.Stderr
	cmdFactory.IO.StdOut = os.Stdout
	rootCmd := commands.NewCmdRoot(cmdFactory, "", "")

	items := gatherFlags(rootCmd)
	slices.SortFunc(items, func(a item, b item) int {
		return cmp.Compare(a.Flag, b.Flag)
	})

	var o output
	o.Items = items
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&o); err != nil {
		log.Printf("err: could not encode json table: %s", err)
	}
}

type output struct {
	Items []item `json:"items"`
}

type item struct {
	Flag        string `json:"flag"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

func gatherFlags(cmd *cobra.Command) []item {
	var items []item

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		items = append(items, item{
			Command:     resolveCommandName(cmd),
			Flag:        f.Name,
			Description: f.Usage,
		})
	})

	for _, cmd := range cmd.Commands() {
		items = append(items, gatherFlags(cmd)...)
	}

	return items
}

func resolveCommandName(cmd *cobra.Command) string {
	names := []string{cmd.Name()}
	cmd.VisitParents(func(parent *cobra.Command) {
		names = append(names, parent.Name())
	})
	slices.Reverse(names)

	return strings.Join(names, " ")
}
