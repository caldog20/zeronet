package cmd

import (
	"context"
	"log"
	"time"

	nodev1 "github.com/caldog20/zeronet/proto/gen/node/v1"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "runs node service",
		Run: func(cmd *cobra.Command, args []string) {
			dialCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			conn, err := grpc.DialContext(dialCtx, "127.0.0.1:55000", grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatal(err)
			}

			client := nodev1.NewNodeServiceClient(conn)
			login, err := client.Login(context.Background(), &nodev1.LoginRequest{AccessToken: ""})
			if err != nil {
				log.Fatal(err)
			}
			log.Println(login.GetStatus())

			up, err := client.Up(context.Background(), &nodev1.UpRequest{})
			if err != nil {
				log.Fatal(err)
			}
			log.Println(up.GetStatus())
		},
	}

	return cmd
}
