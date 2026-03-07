package cli

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type HelpJSON struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Commands    []CommandHelp `json:"commands"`
}

type CommandHelp struct {
	Name        string            `json:"name"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Usage       string            `json:"usage"`
	Arguments   []ArgHelp         `json:"arguments,omitempty"`
	Flags       []FlagHelp        `json:"flags,omitempty"`
	Examples    []string          `json:"examples,omitempty"`
	ExitCodes   map[string]string `json:"exit_codes,omitempty"`
}

type ArgHelp struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

type FlagHelp struct {
	Name        string `json:"name"`
	Short       string `json:"short,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

func GenerateHelpJSON(root *cobra.Command) *HelpJSON {
	h := &HelpJSON{
		Name:        root.Name(),
		Version:     Version,
		Description: root.Long,
	}

	for _, cmd := range root.Commands() {
		if cmd.Hidden || cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		h.Commands = append(h.Commands, buildCommandHelp(cmd))
	}

	return h
}

func buildCommandHelp(cmd *cobra.Command) CommandHelp {
	ch := CommandHelp{
		Name:        cmd.Name(),
		Summary:     cmd.Short,
		Description: cmd.Long,
		Usage:       cmd.UseLine(),
	}

	// Parse arguments from Use string (e.g. "import <file.ifc>")
	ch.Arguments = parseArguments(cmd)

	// Extract flags
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		fh := FlagHelp{
			Name:        f.Name,
			Short:       f.Shorthand,
			Type:        f.Value.Type(),
			Description: f.Usage,
		}
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "[]" {
			fh.Default = f.DefValue
		}
		annotations := f.Annotations
		if annotations != nil {
			if _, ok := annotations[cobra.BashCompOneRequiredFlag]; ok {
				fh.Required = true
			}
		}
		ch.Flags = append(ch.Flags, fh)
	})

	// Parse examples
	if cmd.Example != "" {
		for _, line := range strings.Split(cmd.Example, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				ch.Examples = append(ch.Examples, trimmed)
			}
		}
	}

	return ch
}

func parseArguments(cmd *cobra.Command) []ArgHelp {
	// Extract arg names from Use string after the command name
	parts := strings.Fields(cmd.Use)
	if len(parts) <= 1 {
		return nil
	}

	var args []ArgHelp
	for _, part := range parts[1:] {
		name := strings.Trim(part, "<>[]")
		required := strings.HasPrefix(part, "<")
		args = append(args, ArgHelp{
			Name:     name,
			Type:     "string",
			Required: required,
		})
	}
	return args
}
