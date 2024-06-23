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

func NewDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "downs node service",
		Run: func(cmd *cobra.Command, args []string) {
			dialCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			conn, err := grpc.DialContext(dialCtx, "127.0.0.1:55000", grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatal(err)
			}

			client := nodev1.NewNodeServiceClient(conn)
			down, err := client.Down(context.Background(), &nodev1.DownRequest{})
			if err != nil {
				log.Fatal(err)
			}
			log.Println(down.GetStatus())
		},
	}

	return cmd
}
