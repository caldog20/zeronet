package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

const (
	ClientID    = "test"
	AuthURL     = "http://blue:8080/realms/test/protocol/openid-connect/auth"
	TokenURL    = "http://blue:8080/realms/test/protocol/openid-connect/token"
	RedirectURI = "http://127.0.0.1:8080/callback"
	Audience    = "zeronet"
	JWKSURL     = "http://blue:8080/realms/test/protocol/openid-connect/certs"
)

type TokenValidator struct {
	kf keyfunc.Keyfunc
}

func NewTokenValidator(ctx context.Context, jwks ...string) (*TokenValidator, error) {
	var jwksURLs []string
	for _, url := range jwks {
		jwksURLs = append(jwksURLs, url)
	}
	kf, err := keyfunc.NewDefaultCtx(ctx, jwksURLs)
	if err != nil {
		return nil, err
	}

	return &TokenValidator{kf: kf}, nil
}

// var oauthConfig = &oauth2.Config{
// 	ClientID: ClientID,
// 	Endpoint: oauth2.Endpoint{
// 		AuthURL:  AuthURL,
// 		TokenURL: TokenURL,
// 	},
// 	RedirectURL: RedirectURI,
// }

func (t *TokenValidator) ValidateAccessToken(token string) error {
	tok, err := jwt.Parse(token, t.kf.Keyfunc)

	if err != nil {
		return fmt.Errorf("error parsing access token: %s", err)
	}

	// Check if the token is valid.
	if !tok.Valid {
		return errors.New("access token is invalid")
	}

	aud, err := tok.Claims.GetAudience()
	if err != nil {
		return err
	}

	if !slices.Contains(aud, Audience) {
		return errors.New("access token audience invalid")
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
