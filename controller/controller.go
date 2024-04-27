package controller

import (
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/caldog20/zeronet/controller/auth"
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

func NewController(db *db.Store, prefix netip.Prefix) *Controller {
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

	err := c.db.UpdatePeer(peer)
	if err != nil {
		return err
	}

	c.currentPeers.Store(peer.ID, true)

	// Handle peer login event here
	//go c.PeerLoginEvent(peer.Copy())
	return nil
}

func (c *Controller) RegisterPeer(req *ctrlv1.LoginPeerRequest) (*types.Peer, error) {
	ip, err := c.db.AllocatePeerIP(c.prefix)
	if err != nil {
		return nil, err
	}

	jwt, err := auth.GenerateJwtWithClaims()
	if err != nil {
		return nil, err
	}

	newPeer := &types.Peer{
		MachineID:      req.GetMachineId(),
		NoisePublicKey: req.GetPublicKey(),
		Hostname:       req.GetHostname(),
		Prefix:         c.prefix.String(),
		IP:             ip,
		Endpoint:       req.GetEndpoint(),
		JWT:            jwt,
	}

	err = c.db.CreatePeer(newPeer)
	if err != nil {
		return nil, errors.New("error creating peer in database")
	}

	return newPeer, nil
}

// func (c *Controller) CreatePeer()
