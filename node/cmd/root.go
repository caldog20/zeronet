package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "node",
		Short: "Overlay Node",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("please use a subcommand or use -h for help")
		},
	}
)

func init() {
	rootCmd.AddCommand(NewRunCommand())
	rootCmd.AddCommand(NewGenerateKeypairCommand())
}

// TODO handle signals and contextual things here
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
