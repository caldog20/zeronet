package cmd

import (
	"log"
	"os"

	_ "github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "",
		Short: "",
		Run: func(cmd *cobra.Command, args []string) {
			svc, err := NewService(&program{})
			if err != nil {
				log.Fatal(err)
			}

			errs := make(chan error, 5)
			logger, err = svc.Logger(errs)
			if err != nil {
				log.Fatal(err)
			}

			go func() {
				for {
					err := <-errs
					if err != nil {
						log.Print(err)
					}
				}
			}()

			err = svc.Run()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(NewUpCommand())
	rootCmd.AddCommand(NewDownCommand())
	rootCmd.AddCommand(NewInstallCommand())
	rootCmd.AddCommand(NewUninstallCommand())
	rootCmd.AddCommand(NewStartCommand())
	rootCmd.AddCommand(NewRunCommand())
	rootCmd.AddCommand(NewStopCommand())
	rootCmd.AddCommand(NewGenerateKeypairCommand())
	rootCmd.AddCommand(NewLoginCommand())

	//rootCmd.PersistentFlags().BoolVar(&profile, "profile", false, "enable pprof profile")
}

// TODO handle signals and contextual things here
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
