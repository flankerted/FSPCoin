package wallettx

import (
	"fmt"
	"math/big"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/core/types_elephant"
	"github.com/contatract/go-contatract/internal/elephantapi"
	"github.com/contatract/go-contatract/log"
)

// signTransactions sets defaults and signs the given transaction
// NOTE: the caller needs to ensure that the nonceLock is held, if applicable,
// and release it after the transaction has been submitted to the tx pool
func SignTransaction(am *accounts.Manager, args elephantapi.SendTxArgs, passphrase string) (*types_elephant.Transaction, error) {
	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: args.From}
	wallet, err := am.Find(account)
	if err != nil {
		log.Error("Find", "err", err)
		return nil, err
	}
	err = args.WalletSetDefaults()
	if err != nil {
		log.Error("WalletSetDefaults", "err", err)
		return nil, err
	}
	// Assemble the transaction and sign with the wallet
	tx := args.WalletToTransaction()

	var chainID *big.Int
	ret, err := wallet.SignTxWithPassphrase(account, passphrase, tx, chainID)
	if err != nil {
		fmt.Println("SignTxWithPassphrase err", err)
		return nil, err
	}

	err = nil
	value, ok := ret.(*types_elephant.Transaction)
	if !ok {
		err = common.ErrInterfaceType
	}
	return value, err
}
