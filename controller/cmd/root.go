package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/caldog20/zeronet/controller"
	"github.com/caldog20/zeronet/controller/auth"
	"github.com/caldog20/zeronet/controller/db"
	"github.com/caldog20/zeronet/controller/middleware"
	controllerv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	"github.com/caldog20/zeronet/third_party"
)

var (
	storePath string
	prefix    string
	autoCert  bool
	grpcPort  uint16
	httpPort  uint16
	// discoveryPort uint16
	debug bool

	// TODO: Refactor: Many components should have the basic setup done in separate functions part of their type
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

			var tokenValidator *auth.TokenValidator = nil

			if !debug {
				tokenValidator, err = auth.NewTokenValidator(ctx)
				if err != nil {
					log.Fatalf("error creating token validator: %s", err)
				}
			}

			// GRPC Server
			grpcServer := controller.NewGRPCServer(ctrl, tokenValidator, !debug)
			server := grpc.NewServer(grpc.UnaryInterceptor(middleware.NewUnaryLogInterceptor()))
			controllerv1.RegisterControllerServiceServer(server, grpcServer)
			reflection.Register(server)

			eg, egCtx := errgroup.WithContext(ctx)

			eg.Go(func() error {
				log.Printf("starting grpc server on port: %d", grpcPort)
				conn, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
				if err != nil {
					return err
				}
				return server.Serve(conn)
			})

			mux := runtime.NewServeMux()

			opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
			err = controllerv1.RegisterControllerServiceHandlerFromEndpoint(
				egCtx,
				mux,
				fmt.Sprintf("localhost:%d", grpcPort),
				opts,
			)

			// This is faster, but disables alot of grpc features including interceptors
			// controllerv1.RegisterControllerServiceHandlerServer(egCtx, mux, grpcServer)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("starting grpc http proxy server on port: %d", httpPort)
			gwServer := &http.Server{
				Addr: fmt.Sprintf(":%d", httpPort),
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if strings.HasPrefix(r.URL.Path, "/api") {
						mux.ServeHTTP(w, r)
						return
					}
					getOpenAPIHandler().ServeHTTP(w, r)
				}),
			}

			eg.Go(func() error {
				if err := gwServer.ListenAndServe(); err != http.ErrServerClosed {
					return err
				}
				return nil
			})
			// Cleanup
			eg.Go(func() error {
				<-egCtx.Done()
				StopGRPCServer(server)
				StopHTTPServer(gwServer)
				return err
			})

			// Wait for all errgroup routines to finish before exiting
			if err = eg.Wait(); err != nil {
				log.Fatal(err)
			}
		},
	}
)

func StopHTTPServer(s *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err := s.Shutdown(ctx)
	if err == context.DeadlineExceeded {
		s.Close()
	}
}

func StopGRPCServer(s *grpc.Server) {
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	t := time.NewTimer(10 * time.Second)
	select {
	case <-t.C:
		s.Stop()
	case <-stopped:
		t.Stop()
	}
}

func getOpenAPIHandler() http.Handler {
	return http.FileServer(http.FS(third_party.OpenAPI))
}

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
