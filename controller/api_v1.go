package controller

import (
	"context"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
