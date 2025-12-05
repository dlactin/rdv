// Package options provides common structs used across different packages
package options

// CmdOptions provides a common set of options used across helm/kustomize and other render-diff commands
type CmdOptions struct {
	Debug      bool
	UpdateDeps bool
	Lint       bool
}
