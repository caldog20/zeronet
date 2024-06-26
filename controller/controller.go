package controller

import (
	"errors"
	"fmt"
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
	prefix netip.Prefix
	// currentPeers sync.Map
	peerChannels sync.Map
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

	if peer.IsDisabled() {
		return errors.New("disabled peer cannot log in")
	}

	peer.LastLogin = time.Now()
	peer.NoisePublicKey = req.GetPublicKey() // TODO: Validate public key
	peer.Hostname = req.GetHostname()
	peer.Endpoint = req.GetEndpoint()
	peer.Connected = false
	peer.LoggedIn = true

	// Update peer in database
	err := c.db.UpdatePeer(peer)
	if err != nil {
		return err
	}
	// Add to map of current peers logged in
	// c.currentPeers.Store(peer.ID, true)

	// Handle peer login event here
	//go c.PeerLoginEvent(peer.Copy())
	return nil
}

// func (c *Controller) DisablePeer(peer *types.Peer) error {
//
// }

func (c *Controller) LogoutPeer(peer *types.Peer) error {
	peer.Connected = false
	peer.LoggedIn = false

	// Logout Peer
	c.PeerForcedLogoutEvent(peer.ID)
	go func() {
		// Wait for 10 seconds before cleaning up channel
		time.Sleep(time.Second * 10)
		c.DeletePeerUpdateChannel(peer.ID)
	}()

	// Handle per logout event here
	// go c.PeerLogoutEvent(peer.Copy())

	err := c.db.UpdatePeer(&types.Peer{ID: peer.ID, LoggedIn: false, Connected: false})
	if err != nil {
		return err
	}
	return nil
}

func (c *Controller) DeletePeer(peerID uint32) error {
	peer := c.db.GetPeerbyID(peerID)
	if peer == nil {
		return errors.New("peer doesn't exist")
	}

	if peer.Connected {
		err := c.LogoutPeer(peer)
		if err != nil {
			return err
		}
	}

	err := c.db.DeletePeer(peer)
	if err != nil {
		return err
	}

	fmt.Println(err)

	return nil
}

func (c *Controller) RegisterPeer(
	req *ctrlv1.LoginPeerRequest,
	userID string,
) (*types.Peer, error) {
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
		Connected:      false,
		LoggedIn:       true,
		LastAuth:       time.Now(),
		LastLogin:      time.Now(),
		User:           userID,
	}

	err = c.db.CreatePeer(newPeer)
	if err != nil {
		return nil, errors.New("error creating peer in database")
	}

	return newPeer, nil
}

// func (c *Controller) CreatePeer()
