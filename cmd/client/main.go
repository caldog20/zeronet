package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

var tok *oauth2.Token

// cleanup closes the HTTP server
func cleanup(server *http.Server) {
	// we run this as a goroutine so that this function falls through and
	// the socket to the browser gets flushed/closed before the server goes away
	go server.Close()
}

func doAuthFlow(info *ctrlv1.GetPKCEAuthInfoResponse) {
	conf := &oauth2.Config{
		ClientID: info.GetClientId(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  info.GetAuthEndpoint(),
			TokenURL: info.GetTokenEndpoint(),
		},
		RedirectURL: info.GetRedirectUri(),
	}

	verifier := oauth2.GenerateVerifier()
	auth_url := conf.AuthCodeURL(
		"state",
		oauth2.S256ChallengeOption(verifier))

	server := &http.Server{Addr: info.GetRedirectUri()}
	// define a handler that will get the authorization code, call the token endpoint, and close the HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// get the authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Println("Url Param 'code' is missing")
			io.WriteString(w, "Error: could not find 'code' URL parameter\n")

			// close the HTTP server and return
			cleanup(server)
			return
		}
		var err error
		ctx := context.Background()
		tok, err = conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			log.Printf("error exchanging access code for tokens: %s\n", err)
			cleanup(server)
			return
		}

		// fmt.Println(tok.AccessToken)

		io.WriteString(w, `
      <html>
			<body>
				<h1>Login successful!</h1>
				<h2>You can close this window and return</h2>
			</body>
		</html>`)

		// close the HTTP server
		cleanup(server)
	})

	// parse the redirect URL for the port number
	u, err := url.Parse(info.GetRedirectUri())
	if err != nil {
		fmt.Printf("bad redirect URL: %s\n", err)
		os.Exit(1)
	}

	// set up a listener on the redirect port
	port := fmt.Sprintf(":%s", u.Port())
	l, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("can't listen to port %s: %s\n", port, err)
		os.Exit(1)
	}

	// open a browser window to the authorizationURL
	err = open.Start(auth_url)
	if err != nil {
		fmt.Printf("can't open browser to URL %s: %s\n", auth_url, err)
		os.Exit(1)
	}

	// start the blocking web server loop
	// this will exit when the handler gets fired and calls server.Close()
	server.Serve(l)
}

func main() {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	gconn, err := grpc.DialContext(
		ctx,
		"127.0.0.1:50000",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal("error connecting to controller grpc: ", err)
	}

	controller := ctrlv1.NewControllerServiceClient(gconn)

	ctx = context.Background()
	// mdCtx := metadata.NewOutgoingContext(
	// 	ctx,
	// 	metadata.New(map[string]string{"authorization": tok.AccessToken}),
	// )
	login, err := controller.LoginPeer(ctx, &ctrlv1.LoginPeerRequest{
		MachineId: "TestMachineID",
		PublicKey: "TestPublicKey",
		Hostname:  "Testmachine",
		Endpoint:  "1.1.1.1:44444",
	})

	if err != nil {
		s, _ := status.FromError(err)
		fmt.Println(err)
		if s.Code() == codes.Unauthenticated {
			resp, err := controller.GetPKCEAuthInfo(
				context.TODO(),
				&ctrlv1.GetPKCEAuthInfoRequest{},
			)
			if err != nil {
				log.Fatal(err)
			}

			doAuthFlow(resp)
			login, err := controller.LoginPeer(ctx, &ctrlv1.LoginPeerRequest{
				MachineId:   "TestMachineID",
				PublicKey:   "TestPublicKey",
				Hostname:    "Testmachine",
				Endpoint:    "1.1.1.1:44444",
				AccessToken: tok.AccessToken,
			})
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(login)

		}
	} else {
		fmt.Println("peer logged in without access token")
		fmt.Println(login)
	}

}
