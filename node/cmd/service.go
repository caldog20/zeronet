package cmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"time"

	"github.com/caldog20/zeronet/node"
	nodev1 "github.com/caldog20/zeronet/proto/gen/node/v1"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

var (
	controller string
	port       uint16
	logger     service.Logger
)

type program struct {
	node   *node.Node
	server *grpc.Server
	// done   chan struct{}
	conn net.Listener
}

func (p *program) Start(s service.Service) error {
	n, err := node.NewNode(controller, port)
	if err != nil {
		log.Fatal(err)
	}

	server := grpc.NewServer()
	nodev1.RegisterNodeServiceServer(server, n)

	p.server = server
	p.node = n
	// p.done = make(chan struct{})

	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	err := p.node.Stop()
	p.server.Stop()
	return err
}

func (p *program) run() {
	conn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 55000))
	if err != nil {
		//logger.Error(err)
		log.Fatal(err)
		return
	}
	if err := p.server.Serve(conn); !errors.Is(err, grpc.ErrServerStopped) {
		logger.Error(err)
	}
}

func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "start service",
		Long:  "",
		Run: func(cmd *cobra.Command, args []string) {
			// fmt.Println("please use a subcommand or use -h for help")

			prg := &program{}

			svc, err := NewService(prg)
			if err != nil {
				log.Fatal(err)
			}

			if err := svc.Start(); err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.PersistentFlags().
		StringVar(&controller, "controller", "127.0.0.1:50000", "controller address in <ip:port> format")
	cmd.PersistentFlags().
		Uint16Var(&port, "port", 0, "listen port for udp socket - defaults to 0 for randomly selected port")
	return cmd
}

// TODO Fix arguments for service when providing argument for controller address
func NewService(program service.Interface) (service.Service, error) {
	options := make(service.KeyValue)
	options["SuccessExitStatus"] = "1 2 8 SIGKILL"
	if runtime.GOOS == "windows" {
		options["OnFailure"] = "restart"
	}

	svcConfig := &service.Config{
		Name:        "node",
		DisplayName: "Zeronet",
		Description: "Zeronet",
		Option:      options,
		Arguments: []string{
			"run",
			"--controller",
			controller,
		},
	}

	s, err := service.New(program, svcConfig)
	return s, err
}

func NewUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "runs node service",
		Run: func(cmd *cobra.Command, args []string) {
			client, close := getManagementClient()
			defer close()

			if err := up(client); err != nil {
				log.Fatal(err)
			}
		},
	}
	return cmd
}

func NewLoginCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "login",
		Run: func(cmd *cobra.Command, args []string) {
			client, close := getManagementClient()
			defer close()

			if err := login(client); err != nil {
				log.Fatal(err)
			}
		},
	}

	return cmd
}

func NewRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "runs the background service",
		Run: func(cmd *cobra.Command, args []string) {
			svc, err := NewService(&program{})
			if err != nil {
				log.Fatal(err)
			}
			err = svc.Run()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
	cmd.PersistentFlags().
		StringVar(&controller, "controller", "127.0.0.1:50000", "controller address in <ip:port> format")
	cmd.PersistentFlags().
		Uint16Var(&port, "port", 0, "listen port for udp socket - defaults to 0 for randomly selected port")

	return cmd
}

func NewInstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "install the background service",
		Run: func(cmd *cobra.Command, args []string) {
			svc, err := NewService(&program{})
			if err != nil {
				log.Fatal(err)
			}
			err = svc.Install()
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.PersistentFlags().
		StringVar(&controller, "controller", "127.0.0.1:50000", "controller address in <ip:port> format")
	cmd.PersistentFlags().
		Uint16Var(&port, "port", 0, "listen port for udp socket - defaults to 0 for randomly selected port")
	return cmd
}

func NewUninstallCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "uninstall the background service",
		Run: func(cmd *cobra.Command, args []string) {
			svc, err := NewService(&program{})
			if err != nil {
				log.Fatal(err)
			}
			err = svc.Uninstall()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
}

func NewStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "stops the background service",
		Run: func(cmd *cobra.Command, args []string) {
			svc, err := NewService(&program{})
			if err != nil {
				log.Fatal(err)
			}
			err = svc.Stop()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
}

func NewDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "downs node service",
		Run: func(cmd *cobra.Command, args []string) {
			client, close := getManagementClient()
			defer close()

			if err := down(client); err != nil {
				log.Fatal(err)
			}
		},
	}

	return cmd
}

func getManagementClient() (nodev1.NodeServiceClient, func()) {
	conn, err := grpc.NewClient(
		"127.0.0.1:55000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}

	client := nodev1.NewNodeServiceClient(conn)

	return client, func() {
		conn.Close()
	}
}

func up(client nodev1.NodeServiceClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	up, err := client.Up(ctx, &nodev1.UpRequest{})
	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			return err
		}
		if st.Code() == codes.PermissionDenied {
			if err := login(client); err != nil {
				return err
			}
			up, err = client.Up(ctx, &nodev1.UpRequest{})
			if err != nil {
				return err
			}
		}
	}

	log.Println(up.GetStatus())
	return nil
}

func down(client nodev1.NodeServiceClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	down, err := client.Down(ctx, &nodev1.DownRequest{})
	if err != nil {
		return err
	}
	log.Println(down.GetStatus())

	return nil
}

func login(client nodev1.NodeServiceClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	login, err := client.Login(ctx, &nodev1.LoginRequest{AccessToken: ""})
	if err != nil {
		return err
	}

	st := login.GetStatus()
	if st == "login successful" {
		log.Println("node login successful")
	} else if st == "need access token" {
		token, err := node.AuthFlow(login)
		if err != nil {
			return err
		}
		login, err = client.Login(ctx, &nodev1.LoginRequest{AccessToken: token})
		if err != nil {
			return err
		}
	}
	return nil
}
