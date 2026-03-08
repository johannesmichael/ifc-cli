package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for ifc-to-db.

To load completions:

Bash:
  $ source <(ifc-to-db completion bash)
  # Or add to ~/.bashrc:
  $ ifc-to-db completion bash > /etc/bash_completion.d/ifc-to-db

Zsh:
  $ ifc-to-db completion zsh > "${fpath[1]}/_ifc-to-db"

Fish:
  $ ifc-to-db completion fish | source
  $ ifc-to-db completion fish > ~/.config/fish/completions/ifc-to-db.fish

PowerShell:
  PS> ifc-to-db completion powershell | Out-String | Invoke-Expression`,
		Example: `  # Generate bash completions
  ifc-to-db completion bash

  # Generate zsh completions
  ifc-to-db completion zsh`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
	return cmd
}
