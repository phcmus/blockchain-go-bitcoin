package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"

	"golang.org/x/crypto/ripemd160"
)

const (
	ChecksumLength = 4
	version        = byte(0x00)
)

type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey  []byte
}

func (w Wallet) Address() []byte {
	// get address of a wallet
	var address []byte

	pubKeyHash := PublicKeyHash(w.PublicKey)
	versionHash := append([]byte{version}, pubKeyHash...) // version payload

	checksum := Checksum(versionHash)

	// address = append(pubKeyHash, checksum...)
	address = append(versionHash, checksum...)

	return Base58Encode(address)
}

func PublicKeyHash(pubKey []byte) []byte {
	sha := sha256.Sum256(pubKey)

	ripemdHasher := ripemd160.New()
	_, err := ripemdHasher.Write(sha[:])
	if err != nil {
		log.Panic(err)
	}
	ripemd160 := ripemdHasher.Sum(nil) // payload

	return ripemd160
}

func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	// generate new ECDSA key pair
	curve := elliptic.P256()

	privKey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	pubKey := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)

	return *privKey, pubKey
}

func MakeWallet() *Wallet {
	// create a new wallet
	priv, pub := NewKeyPair()
	return &Wallet{priv, pub}
}

func Checksum(payload []byte) []byte {
	// payload: version + ripemd160
	first := sha256.Sum256(payload)
	second := sha256.Sum256(first[:])

	return second[:ChecksumLength]
}

func ValidateAddress(address []byte) bool {
	pubKeyHash := Base58Decode(address)
	actualChecksum := pubKeyHash[len(pubKeyHash)-ChecksumLength:]

	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-ChecksumLength]
	targetChecksum := Checksum(append([]byte{version}, pubKeyHash...))

	return bytes.Equal(actualChecksum, targetChecksum)
}
