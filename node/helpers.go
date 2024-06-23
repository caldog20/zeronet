package node

import (
	"cmp"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"os"

	"github.com/flynn/noise"
	"gopkg.in/yaml.v3"
)

type Key struct {
	Public  string `yaml:"PublicKey"`
	Private string `yaml:"PrivateKey"`
}

func GenerateNewKeypair() (noise.DHKey, error) {
	fmt.Println("generating new noise keypair")
	keypair, err := CipherSuite.GenerateKeypair(nil)
	if err != nil {
		return noise.DHKey{}, err
	}
	err = StoreKeyToDisk(keypair)
	if err != nil {
		return noise.DHKey{}, err
	}
	fmt.Println("WARNING! Do not share private key")
	fmt.Println("public key: ", base64.StdEncoding.EncodeToString(keypair.Public))
	fmt.Println("private key: ", base64.StdEncoding.EncodeToString(keypair.Private))
	return keypair, nil
}

func LoadKeyFromDisk() (noise.DHKey, error) {
	var key Key
	var noise noise.DHKey

	keyfile, err := os.Open(os.ExpandEnv("$HOME/overlay.keypair"))
	if err != nil {
		return noise, errors.New("File not found")
	}

	err = yaml.NewDecoder(keyfile).Decode(&key)
	if err != nil {
		return noise, errors.New("error decoding file")
	}

	priv, err := base64.StdEncoding.DecodeString(key.Private)
	if err != nil {
		return noise, errors.New("error decoding private key")
	}
	pub, err := base64.StdEncoding.DecodeString(key.Public)
	if err != nil {
		return noise, errors.New("error decoding public key")
	}

	noise.Public = pub
	noise.Private = priv

	return noise, nil
}

func StoreKeyToDisk(keyPair noise.DHKey) error {
	var key Key

	keyfile, err := os.Create(os.ExpandEnv("$HOME/overlay.keypair"))
	if err != nil {
		return err
	}
	keyfile.Seek(0, 0)

	key.Private = base64.StdEncoding.EncodeToString(keyPair.Private)
	key.Public = base64.StdEncoding.EncodeToString(keyPair.Public)

	err = yaml.NewEncoder(keyfile).Encode(key)
	if err != nil {
		return err
	}

	return nil
}

func CompareAddrPort(p1, p2 netip.AddrPort) int {
	c := p1.Addr().Compare(p2.Addr())
	if c != 0 {
		return c
	}
	return cmp.Compare(p1.Port(), p2.Port())
}

func DecodeBase64Key(key string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func ParseAddr(addr string) (netip.Addr, error) {
	cidr, err := netip.ParsePrefix(addr)
	if err != nil {
		return netip.Addr{}, err
	}
	return cidr.Addr(), nil
}

func ParseAddrPort(ap string) (netip.AddrPort, error) {
	endpoint, err := netip.ParseAddrPort(ap)
	if err != nil {
		return netip.AddrPort{}, err
	}
	return endpoint, nil
}
