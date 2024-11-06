package controller

import (
	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
	log "github.com/sirupsen/logrus"
)

func (c *Controller) GetPeerUpdateChannel(id uint32) chan *ctrlv1.UpdateResponse {
	pc := make(chan *ctrlv1.UpdateResponse)
	c.peerChannels.Store(id, pc)
	return pc
}

func (c *Controller) DeletePeerUpdateChannel(id uint32) {
	pc, ok := c.peerChannels.LoadAndDelete(id)
	if ok {
		close(pc.(chan *ctrlv1.UpdateResponse))
	}
}

func (c *Controller) CloseAllPeerUpdateChannels() {
	// Loop through map and close each existing channel
	// Delete the channel from the map after closing
	c.peerChannels.Range(func(k, v interface{}) bool {
		pc := v.(chan *ctrlv1.UpdateResponse)
		close(pc)
		c.peerChannels.Delete(k.(uint32))
		return true
	})
}

func (c *Controller) GetConnectedPeers(id uint32) (*ctrlv1.PeerList, error) {
	peers, err := c.db.GetConnectedPeers()
	if err != nil {
		return nil, err
	}

	// Loop through and convert to protobuf message
	var peerList []*ctrlv1.Peer

	for _, p := range peers {
		if p.ID != id {
			peerList = append(peerList, p.Proto())
		}
	}

	count := uint32(len(peerList))

	return &ctrlv1.PeerList{
		Peers: peerList,
		Count: count,
	}, nil
}

func (c *Controller) PeerConnectedEvent(id uint32) {
	peer := c.db.GetPeerbyID(id)
	if peer == nil {
		return
	}
	update := &ctrlv1.UpdateResponse{
		UpdateType: ctrlv1.UpdateType_CONNECT,
		PeerList: &ctrlv1.PeerList{
			Count: 1,
			Peers: []*ctrlv1.Peer{peer.Proto()},
		},
	}

	c.peerChannels.Range(func(k, v interface{}) bool {
		peerID := k.(uint32)
		pc := v.(chan *ctrlv1.UpdateResponse)
		if peerID != id {
			pc <- update
		}
		return true
	})
}

func (c *Controller) PeerDisconnectedEvent(id uint32) {
	peer := c.db.GetPeerbyID(id)
	if peer == nil {
		return
	}
	update := &ctrlv1.UpdateResponse{
		UpdateType: ctrlv1.UpdateType_DISCONNECT,
		PeerList: &ctrlv1.PeerList{
			Count: 1,
			Peers: []*ctrlv1.Peer{peer.Proto()},
		},
	}

	c.peerChannels.Range(func(k, v interface{}) bool {
		peerID := k.(uint32)
		pc := v.(chan *ctrlv1.UpdateResponse)
		if peerID != id {
			pc <- update
		}
		return true
	})
}

func (c *Controller) PeerForcedLogoutEvent(id uint32) {
	update := &ctrlv1.UpdateResponse{
		UpdateType: ctrlv1.UpdateType_LOGOUT,
	}

	pc, ok := c.peerChannels.Load(id)
	if ok {
		pc.(chan *ctrlv1.UpdateResponse) <- update
	}
}

func (c *Controller) handleUpdateRequest(reqId uint32, msg *ctrlv1.UpdateRequest) {
	switch msg.UpdateType {
	case ctrlv1.UpdateType_ICE:
		c.handleIceUpdateRequest(reqId, msg.GetIceUpdate())
	default:
		log.Printf("unknown update request type %d from peer %d", msg.UpdateType, reqId)
	}
}

func (c *Controller) handleIceUpdateRequest(reqId uint32, msg *ctrlv1.IceUpdate) {
	switch msg.UpdateType {
	// Ice Offer to Send to remote peer
	case ctrlv1.IceUpdateType_OFFER:
		update := &ctrlv1.UpdateResponse{
			UpdateType: ctrlv1.UpdateType_ICE,
			IceUpdate: &ctrlv1.IceUpdate{
				UpdateType: ctrlv1.IceUpdateType_OFFER,
				PeerId:     reqId,
				Ufrag:      msg.GetUfrag(),
				Pwd:        msg.GetPwd(),
			},
		}
		c.sendPeerUpdate(msg.GetPeerId(), update)
	case ctrlv1.IceUpdateType_ANSWER:
		update := &ctrlv1.UpdateResponse{
			UpdateType: ctrlv1.UpdateType_ICE,
			IceUpdate: &ctrlv1.IceUpdate{
				UpdateType: ctrlv1.IceUpdateType_ANSWER,
				PeerId:     reqId,
				Ufrag:      msg.GetUfrag(),
				Pwd:        msg.GetPwd(),
			},
		}
		c.sendPeerUpdate(msg.GetPeerId(), update)
	case ctrlv1.IceUpdateType_CANDIDATE:
		update := &ctrlv1.UpdateResponse{
			UpdateType: ctrlv1.UpdateType_ICE,
			IceUpdate: &ctrlv1.IceUpdate{
				UpdateType: ctrlv1.IceUpdateType_CANDIDATE,
				PeerId:     reqId,
				Candidate:  msg.GetCandidate(),
			},
		}
		c.sendPeerUpdate(msg.GetPeerId(), update)
	default:
	}
}

// TODO error checking on updates when sending on peer channel
func (c *Controller) sendPeerUpdate(id uint32, update *ctrlv1.UpdateResponse) {
	pc, ok := c.peerChannels.Load(id)
	if ok {
		pc.(chan *ctrlv1.UpdateResponse) <- update
	}
}
