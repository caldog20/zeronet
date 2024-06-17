package types

import (
	"time"

	ctrlv1 "github.com/caldog20/zeronet/proto/gen/controller/v1"
)

type Peer struct {
	ID             uint32 `json:"id"         gorm:"primaryKey,autoIncrement"`
	MachineID      string `json:"machine_id" gorm:"unique,not null"`
	NoisePublicKey string `json:"-"          gorm:"uniqueIndex,not null"`
	IP             string `json:"ip"         gorm:"uniqueIndex"`
	Prefix         string `json:"prefix"     gorm:"not null"`
	Endpoint       string `json:"endpoint"`
	Hostname       string `json:"hostname"`

	LoggedIn bool `json:"logged_in"`
	Connected bool   `json:"connected"`
	User      string `json:"user"`
	Disabled bool `json:"disabled"`
	// JWT      string

	LastLogin time.Time
	LastAuth  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (p *Peer) Copy() *Peer {
	return &Peer{
		p.ID,
		p.MachineID,
		p.NoisePublicKey,
		p.IP,
		p.Prefix,
		p.Endpoint,
		p.Hostname,
		p.Connected,
		p.LoggedIn,
		p.User,
		p.Disabled,
		// p.JWT,
		p.LastLogin,
		p.LastAuth,
		p.CreatedAt,
		p.UpdatedAt,
	}
}

func (p *Peer) Proto() *ctrlv1.Peer {
	return &ctrlv1.Peer{
		// MachineId: p.MachineID,
		Id:        p.ID,
		PublicKey: p.NoisePublicKey,
		Hostname:  p.Hostname,
		Ip:        p.IP,
		Endpoint:  p.Endpoint,
		Prefix:    p.Prefix,
		User:      p.User,
		Connected:  p.Connected,
	}
}

func (p *Peer) ProtoConfig() *ctrlv1.PeerConfig {
	return &ctrlv1.PeerConfig{
		PeerId:   p.ID,
		TunnelIp: p.IP,
		Prefix:   p.Prefix,
	}
}

func (p *Peer) IsLoggedIn() bool {
	return p.LoggedIn
}

func (p *Peer) IsConnected() bool {
	return p.Connected
}

func (p *Peer) IsDisabled() bool {
	return p.Disabled
}

// func (p *Peer) ValidateToken(token string) bool {
// 	if token != p.JWT {
// 		return false
// 	}
// 	if p.IsAuthExpired() {
// 		return false
// 	}
// 	return true
// }

func (p *Peer) IsAuthExpired() bool {
	now := time.Now()
	// get duration passed since last auth until now
	duration := now.Sub(p.LastAuth)

	// Get days from duration
	days := duration.Hours() / 24

	// if 30+ days have passed, auth is expired
	return days >= 30
}

func (p *Peer) UpdateAuth() {
	// Reset last auth to current time
	p.LastAuth = time.Now()
}
