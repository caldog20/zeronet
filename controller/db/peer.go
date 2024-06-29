package db

import (
	"github.com/caldog20/zeronet/controller/types"
)

func (s *Store) GetPeerByMachineID(machineID string) *types.Peer {
	var peer types.Peer
	err := s.db.Where("machine_id = ?", machineID).First(&peer).Error
	if err != nil {
		return nil
	}
	return &peer
}

func (s *Store) GetPeers() ([]types.Peer, error) {
	var peers []types.Peer
	result := s.db.Find(&peers)
	if result.Error != nil {
		return nil, result.Error
	}
	return peers, nil

}

func (s *Store) GetConnectedPeers() ([]types.Peer, error) {
	var peers []types.Peer
	err := s.db.Where("connected = ?", true).Find(&peers).Error
	if err != nil {
		return nil, err
	}
	return peers, nil
}

func (s *Store) GetPeerbyID(id uint32) *types.Peer {
	var peer types.Peer

	err := s.db.First(&peer, id).Error
	if err != nil {
		return nil
	}

	return &peer
}

func (s *Store) SetPeerConnected(peer *types.Peer, connected bool) error {
	return s.db.Model(peer).Update("connected", connected).Error
}

func (s *Store) UpdatePeerEndpoint(id uint32, endpoint string) error {
	return s.db.Model(&types.Peer{ID: id}).Update("endpoint", endpoint).Error
}

func (s *Store) UpdatePeer(peer *types.Peer) error {
	return s.db.Updates(peer).Error
}

func (s *Store) CreatePeer(peer *types.Peer) error {
	return s.db.Create(peer).Error
}

func (s *Store) DeletePeer(peer *types.Peer) error {
	return s.db.Delete(peer).Error
}
