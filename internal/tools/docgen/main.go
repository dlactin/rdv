// This docgen script is used to keep our flags and examples documentation
// updated in the README.md as our command changes
package main

import (
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/dlactin/rdv/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FlagInfo struct {
	Name      string
	Shorthand string
	Usage     string
	Default   string
}

func main() {
	tpl := template.Must(template.ParseFiles("../README.md.tmpl"))

	root := cmd.Root()

	// Include Cobra default flags
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()

	// Parse custom flags for our documentation
	flags := collectFlags(root)

	// Use the root.Use string to format our examples
	// We want to wrap any examples that include our Use string in a code block
	example := formatExamples(root.Example, root.Use)

	// Create our README.md
	out, err := os.Create("../README.md")
	if err != nil {
		panic(err)
	}

	// Provide our flags map and example string to our template
	err = tpl.Execute(out, map[string]any{
		"Flags":   flags,
		"Example": example,
	})
	if err != nil {
		panic(err)
	}

	err = out.Close()
	if err != nil {
		panic(err)
	}
}

// Parse all examples that start with the Use string and wrap
// them in a bash code block for easier parsing
func formatExamples(example, use string) string {
	lines := strings.Split(example, "\n")
	var out []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if the line looks like a command
		if strings.HasPrefix(trimmed, use) {
			// Wrap in a bash code block
			out = append(out, "```bash\n"+trimmed+"\n```")
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func collectFlags(cmd *cobra.Command) []FlagInfo {
	var out []FlagInfo
	seen := make(map[string]bool)

	visitor := func(f *pflag.Flag) {
		// Prevent duplicates if a flag is in both sets
		if seen[f.Name] {
			return
		}
		seen[f.Name] = true

		// Use em dash for empty values
		if f.Shorthand == "" {
			f.Shorthand = "\u2014"
		}

		if f.DefValue == "" {
			f.DefValue = "\u2014"
		}

		out = append(out, FlagInfo{
			Name:      f.Name,
			Shorthand: f.Shorthand,
			Usage:     f.Usage,
			Default:   f.DefValue,
		})
	}

	// Global flags (PersistentFlags)
	cmd.PersistentFlags().VisitAll(visitor)

	cmd.Flags().FlagUsages()

	// Visit Local flags (Flags defined only for this command)
	cmd.Flags().VisitAll(visitor)

	// Sort flags by name so the README table is consistent
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out
}
