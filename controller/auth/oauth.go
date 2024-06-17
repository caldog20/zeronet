package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

type UserInfo struct {
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Email      string `json:"email"`
	Nickname   string `json:"nickname"`
	UpdatedAt time.Time `json:"-"`
}

func (u *UserInfo) NeedsRefresh() bool {
	now := time.Now()
	duration := now.Sub(u.UpdatedAt)

	hours := duration.Hours()

	return hours >= 1
}

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
	userCache	sync.Map
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

	tv := &TokenValidator{
		kf:          kf,
		config:      config,
		clientID:    clientID,
		audience:    audience,
		redirectUri: redirectUri,
	}

	go tv.userInfoCacheRoutine(ctx)

	return tv, nil
}

// Every 5 minutes loop through userinfo cache and clear any data
// older than duration declared in UserInfo.NeedsRefresh()
func (t *TokenValidator) userInfoCacheRoutine(ctx context.Context) {
	go func(ctx context.Context) {
		timer := time.NewTimer(time.Minute * 5)
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				t.userCache.Range(func(k,v interface{}) bool {
					userInfo := v.(*UserInfo)
					if userInfo.NeedsRefresh() {
						t.userCache.Delete(k.(string))
					}
					return true
				})
				timer.Reset(time.Minute * 5)
			}
		}
	}(ctx)
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

func (t *TokenValidator) GetUserInfo(token *jwt.Token) (*UserInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("GET", t.config.UserInfoEndpoint, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.Raw))
	resp, err := client.Do(req)
	if err != nil {
		return &UserInfo{}, err
	}

	info := &UserInfo{}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(info)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func (t *TokenValidator) GetUser(token *jwt.Token) (string, error) {
	sub, err := token.Claims.GetSubject()
	if err != nil {
		return "", err
	}
	
	// Check cache to see if userinfo was already fetched
	userInfo, ok := t.userCache.Load(sub)
	// Userinfo doesn't exist, call userinfo endpoint
	// add userinfo to cache on successful call to userinfo endpoint
	if !ok {
		userInfo, err = t.GetUserInfo(token)
		if err != nil {
			return "", errors.New("error retrieving user info from idp userinfo endpoint")
		}
		t.userCache.Store(sub, userInfo)
	}

	return userInfo.(*UserInfo).Email, nil
}

func (t *TokenValidator) ValidateAccessToken(token string) (string, error) {
	tok, err := jwt.Parse(token, t.kf.Keyfunc)

	if err != nil {
		return "", fmt.Errorf("error parsing access token: %s", err)
	}

	// Check if the token is valid.
	if !tok.Valid {
		return "", errors.New("access token is invalid")
	}

	if err := validateAudience(t.audience, tok); err != nil {
		return "", err
	}

	user, err := t.GetUser(tok)
	if err != nil {
		return "", err
	}
	
	return user, nil
}

func validateAudience(audience string, token *jwt.Token) error {
	audiences, err := token.Claims.GetAudience()
	if err != nil {
		return err
	}

	if !slices.Contains(audiences, audience) {
		return errors.New("invalid audience")
	}

	return nil
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
