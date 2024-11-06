package node

import (
	"log"
	"net"
	"time"

	"github.com/pion/stun"
)

func isStunMessage(message []byte) bool {
	return stun.IsMessage(message)
}

func (node *Node) stunRoutine() {
	go func() {
		t := time.NewTicker(time.Second * 60)
		for {
			select {
			case <-node.runCtx.Done():
				return
			case <-t.C:
				err := node.sendStunRequest()
				if err != nil {
					log.Println("sending stun request failed: " + err.Error())
				}
			}
		}
	}()
}

func (node *Node) sendStunRequest() error {
	m := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	addr, err := net.ResolveUDPAddr("udp4", "stun.l.google.com:19302")
	if err != nil {
		return err
	}

	_, err = node.conn.WriteToUDP(m.Raw, addr)

	return err
}

func (node *Node) handleStunMessage(message []byte) {
	natAddr, err := node.decodeStunMessage(message)
	if err != nil {
		log.Printf("Error decoding stun message: %v", err)
		return
	}

	// TODO revisit this

	node.lock.RLock()
	currentEndpoint := node.discoveredEndpoint
	//id := node.machineID
	node.lock.RUnlock()
	if currentEndpoint != natAddr {
		node.lock.Lock()
		node.discoveredEndpoint = natAddr
		node.lock.Unlock()
		//node.grpcClient.UpdateEndpoint(id, natAddr)
	}

	//log.Println("Got stun message", natAddr)
}

func (node *Node) decodeStunMessage(message []byte) (string, error) {
	m := new(stun.Message)
	m.Raw = message
	err := m.Decode()

	if err != nil {
		return "", err
	}
	var xorAddr stun.XORMappedAddress
	if xorErr := xorAddr.GetFrom(m); xorErr != nil {
		//log.Println("getFrom:", xorErr)
		return "", xorErr
	}

	return xorAddr.String(), nil
}
