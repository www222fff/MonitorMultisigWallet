package main

import (
	"github.com/www222fff/watchUTXO/go-bitcoind"
	"log"
	"time"
       )

const (
	SERVER_HOST        = "127.0.0.1"
	SERVER_PORT        = 8332
	RPCUSER            = "danny"
	RPCPASSWD          = "danny_wang"
	USESSL             = false
	WALLET_NAME        = "danny"
	WALLET_PASSPHRASE  = "test"
      )

var watch_addresses = []string{"btg1qmc6uua0jngs9qr38w3pchcvdcrzu878t8p8nwqtj32rtjvjfvnfqywt5pr"}

func findWallet(slice []string, s string) int {
	for index, value := range slice {
		if value == s {
			return index
		}
	}
	return -1
}

func main() {
	bc1, err := bitcoind.New(SERVER_HOST, SERVER_PORT, "", RPCUSER, RPCPASSWD, USESSL)
	if err != nil {
		log.Fatalln(err)
	}

	wallets, err := bc1.ListWallet()
	r := findWallet(wallets, WALLET_NAME)
	if r == -1 {
		err = bc1.LoadWallet(WALLET_NAME, false)
		log.Println(err)
	}

	bc, err := bitcoind.New(SERVER_HOST, SERVER_PORT, WALLET_NAME, RPCUSER, RPCPASSWD, USESSL)
	if err != nil {
		log.Fatalln(err)
	}

	err = bc.WalletPassphrase(WALLET_PASSPHRASE, 100000000)
	log.Println(err)

	utxoMap := make(map[string]bitcoind.UTXO)

	for {
		//list all utxo of watched multisig address
		utxos, err := bc.ListUnspent(1, 999999, watch_addresses)
		if err != nil {
			log.Fatalln(err)
		}

		//filter delta utxo
		deltaUtxoMap := make(map[string]bitcoind.UTXO)
		for _, utxo := range utxos {
			_, ok := utxoMap[utxo.TxID]
			if (ok) {
				log.Println("existed utxo", utxo)
			} else {
				log.Println("found new utxo", utxo)
				utxoMap[utxo.TxID] = utxo
				deltaUtxoMap[utxo.TxID] = utxo
			}
		}

		//handle new utxo, send deposit event
		for txid := range deltaUtxoMap {
			log.Println(txid)
		}

		time.Sleep(1 * time.Second)
	}

}
