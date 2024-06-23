package node

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	nodev1 "github.com/caldog20/zeronet/proto/gen/node/v1"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

var tok *oauth2.Token

// cleanup closes the HTTP server
func cleanup(server *http.Server) {
	// we run this as a goroutine so that this function falls through and
	// the socket to the browser gets flushed/closed before the server goes away
	go server.Close()
}

func AuthFlow(info *nodev1.LoginResponse) (string, error) {
	conf := &oauth2.Config{
		ClientID: info.GetClientId(),
		Endpoint: oauth2.Endpoint{
			AuthURL:  info.GetAuthEndpoint(),
			TokenURL: info.GetTokenEndpoint(),
		},
		RedirectURL: info.GetRedirectUri(),
		Scopes:      []string{"openid profile email offline_access"},
	}

	verifier := oauth2.GenerateVerifier()
	auth_url := conf.AuthCodeURL(
		"state",
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("audience", info.GetAudience()))

	listenAddr := strings.TrimPrefix(info.GetRedirectUri(), "http://")
	server := http.Server{Addr: listenAddr}
	// define a handler that will get the authorization code, call the token endpoint, and close the HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// get the authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Println("Url Param 'code' is missing")
			io.WriteString(w, "Error: could not find 'code' URL parameter\n")

			// close the HTTP server and return
			cleanup(&server)
			return
		}
		var err error
		ctx := context.Background()

		tok, err = conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
		if err != nil {
			log.Printf("error exchanging access code for tokens: %s\n", err)
			cleanup(&server)
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
		cleanup(&server)

	})

	// parse the redirect URL for the port number
	u, err := url.Parse(info.GetRedirectUri())
	if err != nil {
		return "", fmt.Errorf("bad redirect URL: %s\n", err)
	}

	// set up a listener on the redirect port
	port := fmt.Sprintf(":%s", u.Port())
	l, err := net.Listen("tcp", port)
	if err != nil {
		return "", fmt.Errorf("can't listen to port %s: %s\n", port, err)
	}

	// open a browser window to the authorizationURL
	err = open.Start(auth_url)
	if err != nil {
		//fmt.Printf("can't open browser to URL %s: %s\n", auth_url, err)
		fmt.Printf("open url in browser manually: %s\n", auth_url)
	}

	// start the blocking web server loop
	// this will exit when the handler gets fired and calls server.Close()
	server.Serve(l)
	return tok.AccessToken, nil
}
