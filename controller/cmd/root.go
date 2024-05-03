package cmd

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/caldog20/zeronet/controller"
	"github.com/caldog20/zeronet/controller/auth"
	"github.com/caldog20/zeronet/controller/db"
	"github.com/caldog20/zeronet/controller/middleware"
	controllerv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

var (
	storePath     string
	prefix        string
	autoCert      bool
	grpcPort      uint16
	httpPort      uint16
	discoveryPort uint16
	debug         bool

	rootCmd = &cobra.Command{
		Use:   "controller",
		Short: "ZeroNet Controller",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				sigchan := make(chan os.Signal, 1)
				signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
				log.Printf("Received %v signal, shutting down services\n", <-sigchan)
				cancel()
			}()

			// Current uses logrus default logger with some tweaks
			log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
			if debug {
				log.SetLevel(log.DebugLevel)
			} else {
				log.SetLevel(log.InfoLevel)
			}

			// TODO Implement config stuff/multiple commands
			log.Printf("initializing sqlite store using file: %s", storePath)
			db, err := db.New(storePath, log.WithField("type", "gorm"))
			if err != nil {
				log.Fatal(err)
			}

			pfix, err := netip.ParsePrefix(prefix)
			if err != nil {
				log.Fatalf("error parsing prefix: %s", err)
			}

			ctrl := controller.NewController(db, pfix)

			tokenValidator, err := auth.NewTokenValidator(ctx)
			if err != nil {
				log.Fatalf("error creating token validator: %s", err)
			}
			// GRPC Server
			grpcServer := controller.NewGRPCServer(ctrl, tokenValidator)
			server := grpc.NewServer(grpc.UnaryInterceptor(middleware.NewUnaryLogInterceptor()))
			controllerv1.RegisterControllerServiceServer(server, grpcServer)
			reflection.Register(server)

			httpServer := controller.NewHTTPServer(ctrl)
			eg, egCtx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				log.Printf("starting grpc server on port: %d", grpcPort)
				conn, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
				if err != nil {
					return err
				}
				return server.Serve(conn)
			})

			eg.Go(func() error {
				log.Printf("starting http server on port: %d", httpPort)
				return httpServer.Serve(fmt.Sprintf(":%d", httpPort))
			})

			// Cleanup
			eg.Go(func() error {
				<-egCtx.Done()
				server.GracefulStop()
				httpServer.Close(context.Background())
				return err
			})

			// Wait for all errgroup routines to finish before exiting
			if err = eg.Wait(); err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	rootCmd.PersistentFlags().
		StringVar(&storePath, "storepath", "store.db", "file path for controller store persistence")
	rootCmd.PersistentFlags().
		StringVar(&prefix, "prefix", "100.70.0.0/24", "prefix to use for the overlay network")
	rootCmd.PersistentFlags().
		BoolVar(&autoCert, "autocert", false, "enable autocert for controller")
	rootCmd.PersistentFlags().
		Uint16Var(&grpcPort, "grpcport", 50000, "port to listen for grpc connections")
	rootCmd.PersistentFlags().
		BoolVar(&debug, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().
		Uint16Var(&httpPort, "httpport", 8080, "port to listen for http connections")
}

// TODO handle signals and contextual things here
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
