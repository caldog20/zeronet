package cmd

import (
	"fmt"

	"github.com/caldog20/overlay/node"
	"github.com/spf13/cobra"
)

func NewGenerateKeypairCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "generate's new keypair and saves to disk",
		Run: func(cmd *cobra.Command, args []string) {
			_, err := node.GenerateNewKeypair()
			if err != nil {
				fmt.Println(err)
			}
		},
	}

	return cmd
}
