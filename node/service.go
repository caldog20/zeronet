package node

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/netip"
	"strings"

	controllerv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	nodev1 "github.com/caldog20/zeronet/proto/gen/node/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (n *Node) Up(ctx context.Context, req *nodev1.UpRequest) (*nodev1.UpResponse, error) {
	if err := n.Start(); err != nil {
		if err.Error() == "node is not logged in" {
			return nil, status.Error(codes.PermissionDenied, "node is not logged in")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &nodev1.UpResponse{Status: "node is running"}, nil
}

func (n *Node) Down(ctx context.Context, req *nodev1.DownRequest) (*nodev1.DownResponse, error) {
	err := n.Stop()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &nodev1.DownResponse{Status: "node is stopped"}, nil
}

func (n *Node) Login(ctx context.Context, req *nodev1.LoginRequest) (*nodev1.LoginResponse, error) {
	n.noise.l.RLock()
	pubkey := base64.StdEncoding.EncodeToString(n.noise.keyPair.Public)
	mid := n.machineID
	hostname := n.hostname
	n.noise.l.RUnlock()

	loginRequest := &controllerv1.LoginPeerRequest{
		MachineId:   mid,
		PublicKey:   pubkey,
		Hostname:    hostname,
		AccessToken: req.GetAccessToken(),
	}

	resp, err := n.grpcClient.client.LoginPeer(ctx, loginRequest)
	if err != nil {
		e, ok := status.FromError(err)
		if ok {
			if e.Code() == codes.Unauthenticated {
				info, err := n.grpcClient.client.GetPKCEAuthInfo(context.Background(), &controllerv1.GetPKCEAuthInfoRequest{})
				if err != nil {
					return nil, status.Error(codes.Internal, ("error getting pkce info for auth flow"))
				}
				return &nodev1.LoginResponse{
					Status:        "need access token",
					ClientId:      info.GetClientId(),
					AuthEndpoint:  info.GetAuthEndpoint(),
					TokenEndpoint: info.GetTokenEndpoint(),
					RedirectUri:   info.GetRedirectUri(),
					Audience:      info.GetAudience(),
				}, nil
			}
		} else {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// TODO: Fix prefix for node address
	// Change to netip.Addr and have prefix separate

	n.id = resp.Config.PeerId
	p := strings.Split(resp.Config.Prefix, "/")
	n.ip = netip.MustParsePrefix(fmt.Sprintf("%s/%s", resp.Config.TunnelIp, p[1]))
	n.loggedIn.Store(true)
	return &nodev1.LoginResponse{Status: "login successful"}, nil
}
