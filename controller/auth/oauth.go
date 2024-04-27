package auth

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

const (
	ClientID    = "test"
	AuthURL     = "http://10.170.241.66:8080/realms/test/protocol/openid-connect/auth"
	TokenURL    = "http://10.170.241.66:8080/realms/test/protocol/openid-connect/token"
	RedirectURI = "http://127.0.0.1:8080/callback"
	Audience    = "zeronet"
	JWKSURL     = "http://10.170.241.66:8080/realms/test/protocol/openid-connect/certs"
)

var myKeyFunc keyfunc.Keyfunc

func StartKeyVerifier() {
	var err error
	myKeyFunc, err = keyfunc.NewDefaultCtx(context.Background(), []string{JWKSURL})
	if err != nil {
		log.Fatal(err)
	}

}

// var oauthConfig = &oauth2.Config{
// 	ClientID: ClientID,
// 	Endpoint: oauth2.Endpoint{
// 		AuthURL:  AuthURL,
// 		TokenURL: TokenURL,
// 	},
// 	RedirectURL: RedirectURI,
// }

func ValidatePKCEAccessToken(token string) error {
	tok, err := jwt.Parse(token, myKeyFunc.Keyfunc)

	if err != nil {
		return fmt.Errorf("error parsing access token: %s", err)
	}

	// Check if the token is valid.
	if !tok.Valid {
		return errors.New("access token is invalid")
	}
	return nil
}

func GetPKCEAuthInfo() *ctrlv1.GetPKCEAuthInfoResponse {
	return &ctrlv1.GetPKCEAuthInfoResponse{
		ClientId:      ClientID,
		AuthEndpoint:  AuthURL,
		TokenEndpoint: TokenURL,
		RedirectUri:   RedirectURI,
		Audience:      Audience,
	}
}
