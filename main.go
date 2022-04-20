package main

import (
	"github.com/www222fff/monitorMultisigWalletGo/bitcoind"
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
	WALLET_PASSPHRASE  = "danny_passphrase"
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

	utxoMap := make(map[string]bitcoind.Transaction)

	for {
		txs, err := bc.ListUnspent(1, 999999, watch_addresses)
		log.Println(err, txs)

		deltaUtxoMap := make(map[string]bitcoind.Transaction)
		for _, tx := range txs {
			_, ok := utxoMap[tx.TxID]
			if (ok) {
				log.Println("found existed utxo")
			} else {
				log.Println("found new utxo")
				utxoMap[tx.TxID] = tx
				deltaUtxoMap[tx.TxID] = tx
			}
		}

		for txid := range deltaUtxoMap {
			//send deposit event???
			log.Println(txid)
		}

		time.Sleep(1 * time.Second)
	}

}
