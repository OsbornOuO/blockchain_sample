package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log"

	"golang.org/x/crypto/ripemd160"
)

const (
	checksumLength = 4
	version        = byte(0x00)
)

// Wallet ...
//                                                                                                             -> version   \
// private key -> ecdsa -> public key -> sha256 -> ripemd160 -> public key hash ---------------------------------------------> base 58 -> address
//                                                                             |-> sha256 -> sha256 -> 4 bytes (take out first 4 bytes in hash) -> check sum /
type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	Publickey  []byte
}

func (w Wallet) Address() []byte {
	pubHash := PublicKeyHash(w.Publickey)
	fmt.Println(pubHash)
	versionedHash := append([]byte{version}, pubHash...)
	checksum := CheckSum(versionedHash)

	fullHash := append(versionedHash, checksum...)
	address := Base58Encode(fullHash)

	fmt.Printf("pub key: %x\n", w.Publickey)
	fmt.Printf("pub hash: %x\n", pubHash)
	fmt.Printf("address: %x\n", address)

	return address
}

func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()

	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	pub := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)

	return *private, pub
}

func MakeWallet() *Wallet {
	private, public := NewKeyPair()
	w := Wallet{private, public}
	return &w
}

func PublicKeyHash(pubKey []byte) []byte {
	pubHash := sha256.Sum256(pubKey)

	hasher := ripemd160.New()
	_, err := hasher.Write(pubHash[:])
	if err != nil {
		log.Panic(err)
	}

	publicRipMD := hasher.Sum(nil)

	return publicRipMD
}

func CheckSum(payload []byte) []byte {
	firshHash := sha256.Sum256(payload)
	secondHash := sha256.Sum256(firshHash[:])

	return secondHash[:checksumLength]
}
