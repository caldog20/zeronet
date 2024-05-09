package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

type OpenIDConfig struct {
	Issuer           string   `json:"issuer"`
	AuthEndpoint     string   `json:"authorization_endpoint"`
	TokenEndpoint    string   `json:"token_endpoint"`
	JWKSEndpoint     string   `json:"jwks_uri"`
	Scopes           []string `json:"scopes_supported"`
	Claims           []string `json:"claims_supported"`
	UserInfoEndpoint string   `json:"userinfo_endpoint"`
}

type TokenValidator struct {
	kf          keyfunc.Keyfunc
	config      *OpenIDConfig
	clientID    string
	audience    string
	redirectUri string
}

func NewTokenValidator(ctx context.Context) (*TokenValidator, error) {
	config, err := getOpenIDConfiguration(os.Getenv("OPENID_CONFIG_URL"))
	if err != nil {
		return nil, err
	}

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{config.JWKSEndpoint})
	if err != nil {
		return nil, err
	}

	clientID := os.Getenv("OPENID_CLIENT_ID")
	audience := os.Getenv("OPENID_AUDIENCE")
	redirectUri := os.Getenv("OPENID_CALLBACK_URL")

	return &TokenValidator{
		kf:          kf,
		config:      config,
		clientID:    clientID,
		audience:    audience,
		redirectUri: redirectUri,
	}, nil
}

func getOpenIDConfiguration(url string) (*OpenIDConfig, error) {
	config := &OpenIDConfig{}
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (t *TokenValidator) GetUserInfo(token *jwt.Token) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", t.config.UserInfoEndpoint, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.Raw))
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	info := struct {
		Name       string `json:"name"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Email      string `json:"email"`
		Nickname   string `json:"nickname"`
	}{}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&info)

	return info.Name, nil
}

// var oauthConfig = &oauth2.Config{
// 	ClientID: ClientID,
// 	Endpoint: oauth2.Endpoint{
// 		AuthURL:  AuthURL,
// 		TokenURL: TokenURL,
// 	},
// 	RedirectURL: RedirectURI,
// }

func (t *TokenValidator) ValidateAccessToken(token string) (string, error) {
	tok, err := jwt.Parse(token, t.kf.Keyfunc)

	if err != nil {
		return "", fmt.Errorf("error parsing access token: %s", err)
	}

	// Check if the token is valid.
	if !tok.Valid {
		return "", errors.New("access token is invalid")
	}

	username, err := t.GetUserInfo(tok)
	if err != nil {
		return "", nil
	}

	aud, err := tok.Claims.GetAudience()
	if err != nil {
		return "", err
	}

	if !slices.Contains(aud, t.audience) {
		return "", errors.New("access token audience invalid")
	}
	return username, nil
}

func (t *TokenValidator) GetPKCEAuthInfo() *ctrlv1.GetPKCEAuthInfoResponse {
	return &ctrlv1.GetPKCEAuthInfoResponse{
		ClientId:      t.clientID,
		AuthEndpoint:  t.config.AuthEndpoint,
		TokenEndpoint: t.config.TokenEndpoint,
		RedirectUri:   t.redirectUri,
		Audience:      t.audience,
	}
}
