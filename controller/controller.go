package controller

import (
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/caldog20/zeronet/controller/db"
	"github.com/caldog20/zeronet/controller/types"
	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

type Controller struct {
	db *db.Store

	// Config Related Items
	prefix       netip.Prefix
	currentPeers sync.Map
}

func NewController(
	db *db.Store,
	prefix netip.Prefix,
) *Controller {
	return &Controller{db: db, prefix: prefix}
}

func (c *Controller) ProcessPeerLogin(peer *types.Peer, req *ctrlv1.LoginPeerRequest) error {
	// peer := c.db.GetPeerByMachineID(req.GetMachineId())
	// if peer == nil {
	// 	return nil, nil
	// }

	peer.LastLogin = time.Now()
	peer.NoisePublicKey = req.GetPublicKey() // TODO: Validate public key
	peer.Hostname = req.GetHostname()
	peer.Endpoint = req.GetEndpoint()
	peer.LoggedIn = true

	// Update peer in database
	err := c.db.UpdatePeer(peer)
	if err != nil {
		return err
	}
	// Add to map of current peers logged in
	c.currentPeers.Store(peer.ID, true)

	// Handle peer login event here
	//go c.PeerLoginEvent(peer.Copy())
	return nil
}

func (c *Controller) LogoutPeer(peer *types.Peer) error {
	peer.LoggedIn = false
	c.currentPeers.Delete(peer.ID)

	// Handle per logout event here
	// go c.PeerLogoutEvent(peer.Copy())

	err := c.db.UpdatePeer(&types.Peer{ID: peer.ID, LoggedIn: false})
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) RegisterPeer(req *ctrlv1.LoginPeerRequest) (*types.Peer, error) {
	ip, err := c.db.AllocatePeerIP(c.prefix)
	if err != nil {
		return nil, err
	}

	// jwt, err := auth.GenerateJwtWithClaims()
	// if err != nil {
	// 	return nil, err
	// }

	newPeer := &types.Peer{
		MachineID:      req.GetMachineId(),
		NoisePublicKey: req.GetPublicKey(),
		Hostname:       req.GetHostname(),
		Prefix:         c.prefix.String(),
		IP:             ip,
		Endpoint:       req.GetEndpoint(),
		LoggedIn:       false,
		LastAuth:       time.Now(),
	}

	err = c.db.CreatePeer(newPeer)
	if err != nil {
		return nil, errors.New("error creating peer in database")
	}

	return newPeer, nil
}

// func (c *Controller) CreatePeer()
