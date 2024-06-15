package controller

import (
	"context"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/caldog20/zeronet/controller/auth"
	"github.com/caldog20/zeronet/controller/types"
	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

type GRPCServer struct {
	controller *Controller
	ctrlv1.UnimplementedControllerServiceServer
	tokenValidator *auth.TokenValidator
}

func NewGRPCServer(controller *Controller, validator *auth.TokenValidator) *GRPCServer {
	return &GRPCServer{controller: controller, tokenValidator: validator}
}

func (s *GRPCServer) GetPKCEAuthInfo(
	ctx context.Context,
	req *ctrlv1.GetPKCEAuthInfoRequest,
) (*ctrlv1.GetPKCEAuthInfoResponse, error) {
	return s.tokenValidator.GetPKCEAuthInfo(), nil
}

func (s *GRPCServer) LoginPeer(
	ctx context.Context,
	req *ctrlv1.LoginPeerRequest,
) (*ctrlv1.LoginPeerResponse, error) {
	// Validate machine ID
	if !validateMachineID(req.GetMachineId()) {
		log.Debugf("invalid or no machine id in request. got: %s", req.GetMachineId())
		return nil, status.Error(codes.InvalidArgument, "invalid machine ID")
	}

	var peer *types.Peer = nil
	var err error
	// Lookup if peer exists/is registered
	peer = s.controller.db.GetPeerByMachineID(req.GetMachineId())
	if peer != nil {
		// Peer already registered, validate and log in
		// if peer.MachineID != req.GetMachineId() {
		// 	log.Debugf(
		// 		"mismatched machine ID for peer. got: %s - expected: %s",
		// 		req.GetMachineId(),
		// 		peer.MachineID,
		// 	)
		// 	return nil, status.Error(codes.InvalidArgument, "machine ID mismatch for peer")
		// }

		// Check peer auth hasn't expired
		if peer.IsAuthExpired() {
			log.Debugf("peer %s auth is expired", peer.MachineID)

			// Validate Access Token for reauthenticating peer
			if _, err = s.validateAccessToken(req.GetAccessToken()); err != nil {
				log.Debugf("peer %s access token is invalid", peer.MachineID)
				if peer.IsLoggedIn() {
					s.controller.LogoutPeer(peer)
				}
				return nil, err
			}

			// Access Token was validated, update peer LastAuth now before Login attempt
			log.Debugf("peer %s reauth successful", peer.MachineID)
			peer.UpdateAuth()
		}

		// Process peer login
		log.Debugf("peer %s login processing", peer.MachineID)
		err := s.controller.ProcessPeerLogin(peer, req)
		if err != nil {
			log.Debugf("peer %s login failed: %s", peer.MachineID, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		// Peer was not found, try to register if access token is present/valid
		log.Debugf("peer registration processing")
		userId, err := s.validateAccessToken(req.GetAccessToken())
		if err != nil {
			log.Debugf("peer registration failed. invalid access token")
			return nil, err
		}
		// Access token is valid, register peer
		peer, err = s.controller.RegisterPeer(req, userId)
		if err != nil {
			log.Debugf("peer registration failed: %s", err)
			return nil, status.Error(codes.Internal, "error registering peer")
		}
	}
	log.Debugf("LoginPeer method completed")
	return &ctrlv1.LoginPeerResponse{Config: peer.ProtoConfig()}, nil
}

func (s *GRPCServer) validateAccessToken(token string) (string, error) {
	// errMsg := "peer registration failed, access token is invalid"
	// if reauth {
	// 	errMsg = "peer reauth failed, access token is invalid"
	// }

	userId, err := s.tokenValidator.ValidateAccessToken(token)
	if err != nil {
		return "", status.Error(
			codes.Unauthenticated,
			err.Error(),
		)
	}
	return userId, nil
}

func (s *GRPCServer) GetPeers(
	ctx context.Context,
	req *ctrlv1.GetPeersRequest,
) (*ctrlv1.GetPeersResponse, error) {

	token, err := extractTokenMetadata(ctx)
	if err != nil {
		return nil, err
	}
	_, err = s.validateAccessToken(token)
	if err != nil {
		return nil, err
	}

	peers, err := s.controller.db.GetPeers()
	if err != nil {
		return nil, status.Error(codes.Internal, "error getting peers from database")
	}

	var p []*ctrlv1.Peer
	for _, peer := range peers {
		p = append(p, peer.Proto())
	}

	return &ctrlv1.GetPeersResponse{Peers: p}, nil
}

func (s *GRPCServer) DeletePeer(
	ctx context.Context,
	req *ctrlv1.DeletePeerRequest,
) (*ctrlv1.DeletePeerResponse, error) {

	token, err := extractTokenMetadata(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.validateAccessToken(token)
	if err != nil {
		return nil, err
	}

	err = s.controller.DeletePeer(req.GetPeerId())
	if err != nil {
		// TODO: Fix this
		if err.Error() == "peer doesn't exist" {
			return nil, status.Error(codes.NotFound, "peer not found")
		}
		return nil, status.Error(codes.Internal, "error deleting peer")
	}

	return &ctrlv1.DeletePeerResponse{}, nil
}
