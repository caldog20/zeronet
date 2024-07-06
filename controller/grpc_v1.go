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
	authEnabled    bool
}

func NewGRPCServer(controller *Controller, validator *auth.TokenValidator, enableAuth bool) *GRPCServer {
	return &GRPCServer{controller: controller, tokenValidator: validator, authEnabled: enableAuth}
}

func (s *GRPCServer) GetPKCEAuthInfo(
	ctx context.Context,
	req *ctrlv1.GetPKCEAuthInfoRequest,
) (*ctrlv1.GetPKCEAuthInfoResponse, error) {
	return s.tokenValidator.GetPKCEAuthInfo(), nil
}

func (s *GRPCServer) UpdateEndpoint(ctx context.Context, req *ctrlv1.UpdateEndpointRequest) (*ctrlv1.UpdateEndpointResponse, error) {
	if !validateMachineID(req.GetMachineId()) {
		log.Debugf("invalid or no machine id in request. got: %s", req.GetMachineId())
		return nil, status.Error(codes.InvalidArgument, "invalid machine ID")
	}

	peer := s.controller.db.GetPeerByMachineID(req.GetMachineId())

	if peer == nil {
		return nil, status.Error(codes.NotFound, "peer not found")
	}

	if peer.IsDisabled() {
		return nil, status.Error(codes.Internal, "peer is currently disabled")
	}

	if !peer.IsLoggedIn() {
		return nil, status.Error(codes.Internal, "peer requires login first")
	}

	if peer.IsAuthExpired() {
		return nil, status.Error(codes.PermissionDenied, "peer auth expired, login again")
	}

	if req.GetEndpoint() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty endpoint")
	}

	err := s.controller.db.UpdatePeerEndpoint(peer.ID, req.GetEndpoint())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update endpoint")
	}

	return &ctrlv1.UpdateEndpointResponse{}, nil
}

func (s *GRPCServer) Punch(ctx context.Context, req *ctrlv1.PunchRequest) (*ctrlv1.PunchResponse, error) {
	if !validateMachineID(req.GetMachineId()) {
		return nil, status.Error(codes.InvalidArgument, "invalid machine ID")
	}

	peer := s.controller.db.GetPeerByMachineID(req.GetMachineId())
	if peer == nil {
		return nil, status.Error(codes.NotFound, "peer not found")
	}

	if peer.IsDisabled() {
		return nil, status.Error(codes.Internal, "peer is currently disabled")
	}

	if !peer.IsLoggedIn() {
		return nil, status.Error(codes.Internal, "peer requires login first")
	}

	if peer.IsAuthExpired() {
		return nil, status.Error(codes.PermissionDenied, "peer auth expired, login again")
	}

	if req.GetEndpoint() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty endpoint")
	}

	s.controller.PeerPunchRequest(req.GetDstPeerId(), req.GetEndpoint())

	return &ctrlv1.PunchResponse{}, nil
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
		if peer.Disabled {
			log.Debugf("peer %s is disabled", peer.MachineID)
			return nil, status.Error(codes.PermissionDenied, "peer is disabled")
		}
		// Check peer auth hasn't expired
		if peer.IsAuthExpired() {
			log.Debugf("peer %s auth is expired", peer.MachineID)

			// Validate Access Token for reauthenticating peer
			user, err := s.validateAccessToken(req.GetAccessToken())
			if err != nil {
				log.Debugf("peer %s access token is invalid", peer.MachineID)
				if peer.IsLoggedIn() {
					s.controller.LogoutPeer(peer)
				}
				return nil, err
			}
			// Ensure token is from user peer belongs to
			if peer.User != user {
				return nil, status.Error(codes.PermissionDenied, "cannot login a peer that belongs to another user - delete the peer and re-register")
			}

			// Access Token was validated, update peer LastAuth now before Login attempt
			peer.UpdateAuth()
			log.Debugf("peer %s reauth successful", peer.MachineID)
		}

		// Process peer login
		log.Debugf("peer %s login processing", peer.MachineID)
		err = s.controller.ProcessPeerLogin(peer, req)
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

func (s *GRPCServer) UpdateStream(req *ctrlv1.UpdateRequest, stream ctrlv1.ControllerService_UpdateStreamServer) error {
	peer := s.controller.db.GetPeerByMachineID(req.GetMachineId())
	if peer == nil {
		return status.Error(codes.NotFound, "peer with machine id is not registered")
	}

	if peer.IsAuthExpired() {
		return status.Error(codes.Unauthenticated, "peer auth is expired, needs new login")
	}

	if peer.IsDisabled() {
		return status.Error(codes.Internal, "peer is currently disabled")
	}

	if !peer.IsLoggedIn() {
		return status.Error(codes.Internal, "peer requires login first")
	}

	log.Printf("peer %d connected to update stream", peer.ID)

	err := s.controller.db.SetPeerConnected(peer, true)
	if err != nil {
		return status.Error(codes.Internal, "error setting peer connected status")
	}

	pc := s.controller.GetPeerUpdateChannel(peer.ID)
	s.controller.PeerConnectedEvent(peer.ID)

	defer func() {
		s.controller.DeletePeerUpdateChannel(peer.ID)
		s.controller.db.SetPeerConnected(peer, false)
		s.controller.PeerDisconnectedEvent(peer.ID)
	}()

	connectedPeers, err := s.controller.GetConnectedPeers(peer.ID)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	initialPeerList := &ctrlv1.UpdateResponse{
		UpdateType: ctrlv1.UpdateType_INIT,
		PeerList:   connectedPeers,
	}

	err = stream.Send(initialPeerList)
	if err != nil {
		log.Printf("peer %d error sending data on stream", peer.ID)
		return status.Error(codes.Internal, "error sending data on stream")
	}

	for {
		select {
		case <-stream.Context().Done():
			// Client disconnected, send event and cleanup
			log.Printf("peer %d disconnected from update stream", peer.ID)
			return nil
		case update, ok := <-pc:
			if !ok {
				// channel is closed, exit
				log.Printf("peer %d channel closed, stopping stream", peer.ID)
				return status.Error(codes.Aborted, "server closed update stream")
			}
			if update != nil {
				err = stream.Send(update)
				if err != nil {
					log.Printf("peer %d error sending data on stream", peer.ID)
					// return status.Error(codes.Internal, "error sending data on stream")
				}
			}
		}
	}
}

// //////////////////////////////
// GRPC Gateway API Methods
// //////////////////////////////
// TODO Should check if user is admin otherwise only return peers user has registered
// Part of plan to add scopes/permissions
func (s *GRPCServer) GetPeer(ctx context.Context, req *ctrlv1.GetPeerRequest) (*ctrlv1.GetPeerResponse, error) {
	if req.GetPeerId() == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid peer id")
	}

	peer := s.controller.db.GetPeerbyID(req.GetPeerId())
	if peer == nil {
		return nil, status.Error(codes.NotFound, "peer not found")
	}

	return &ctrlv1.GetPeerResponse{
		Peer: peer.ProtoDetails(),
	}, nil
}

func (s *GRPCServer) GetPeers(
	ctx context.Context,
	req *ctrlv1.GetPeersRequest,
) (*ctrlv1.GetPeersResponse, error) {

	_, err := s.extractAndValidateToken(ctx)
	if err != nil {
		return nil, err
	}

	peers, err := s.controller.db.GetPeers()
	if err != nil {
		return nil, status.Error(codes.Internal, "error getting peers from database")
	}

	var p []*ctrlv1.PeerDetails
	for _, peer := range peers {
		p = append(p, peer.ProtoDetails())
	}

	return &ctrlv1.GetPeersResponse{Peers: p}, nil
}

func (s *GRPCServer) DeletePeer(
	ctx context.Context,
	req *ctrlv1.DeletePeerRequest,
) (*ctrlv1.DeletePeerResponse, error) {

	_, err := s.extractAndValidateToken(ctx)
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

func (s *GRPCServer) extractAndValidateToken(ctx context.Context) (string, error) {
	if !s.authEnabled {
		return "debug", nil
	}
	token, err := extractTokenMetadata(ctx)
	if err != nil {
		return "", err
	}

	user, err := s.validateAccessToken(token)
	if err != nil {
		return "", err
	}

	return user, nil
}

func (s *GRPCServer) validateAccessToken(token string) (string, error) {
	if !s.authEnabled {
		return "debug", nil
	}

	userId, err := s.tokenValidator.ValidateAccessToken(token)
	if err != nil {
		return "", status.Error(
			codes.Unauthenticated,
			err.Error(),
		)
	}
	return userId, nil
}
