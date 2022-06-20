package wallet

import (
	"log"

	"github.com/mr-tron/base58"
)

func Base58Encode(data []byte) []byte {
	return []byte(base58.Encode(data))
}

func Base58Decode(data []byte) []byte {
	decode, err := base58.Decode(string(data[:]))
	if err != nil {
		log.Panic(err)
	}
	return []byte(decode)
}
