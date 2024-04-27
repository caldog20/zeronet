package controller

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/caldog20/zeronet/controller/auth"
	"github.com/caldog20/zeronet/controller/types"
	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

type GRPCServer struct {
	controller *Controller
	ctrlv1.UnimplementedControllerServiceServer
}

func NewGRPCServer(controller *Controller) *GRPCServer {
	return &GRPCServer{controller: controller}
}

func (s *GRPCServer) GetPKCEAuthInfo(
	ctx context.Context,
	req *ctrlv1.GetPKCEAuthInfoRequest,
) (*ctrlv1.GetPKCEAuthInfoResponse, error) {
	return auth.GetPKCEAuthInfo(), nil
}

func (s *GRPCServer) LoginPeer(
	ctx context.Context,
	req *ctrlv1.LoginPeerRequest,
) (*ctrlv1.LoginPeerResponse, error) {
	// Validate machine ID
	if !validateMachineID(req.GetMachineId()) {
		return nil, status.Error(codes.InvalidArgument, "invalid machine ID")
	}

	var peer *types.Peer = nil
	var err error
	// Lookup if peer exists/is registered
	peer = s.controller.db.GetPeerByMachineID(req.GetMachineId())
	if peer != nil {
		if peer.IsAuthExpired() {
			// check for access token here
			// if valid, generate new JWT for peer and return with peerconfig
			err := auth.ValidatePKCEAccessToken(req.GetAccessToken())
			if err != nil {
				return nil, status.Error(
					codes.Unauthenticated,
					"peer auth is expired, reauth using SSO",
				)
			} else {
				jwt, err := auth.GenerateJwtWithClaims()
				if err != nil {
					return nil, status.Error(codes.Internal, "error generating new JWT token for existing peer")
				}
				peer.JWT = jwt
			}

		}

		// Process peer login
		err := s.controller.ProcessPeerLogin(peer, req)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		// Peer was not found, try to register
		// Validate register token here later
		if err := auth.ValidatePKCEAccessToken(req.GetAccessToken()); err != nil {
			return nil, status.Error(
				codes.Unauthenticated,
				"access token required to register peer",
			)
		}
		// Create/Register the peer
		peer, err = s.controller.RegisterPeer(req)
		if err != nil {
			return nil, status.Error(codes.Internal, "error registering peer")
		}
	}

	return &ctrlv1.LoginPeerResponse{Jwt: peer.JWT, Config: peer.ProtoConfig()}, nil
}
