package gui

import "github.com/contatract/go-contatract/log"

type GUIBackend interface {
	Init() ([]byte, error)

	DoOperation(cmd string, params []interface{}, rate chan int) (interface{}, error)

	GetAccAddresses() []string

	PassphraseEntered() bool

	RemoteIPEntered() bool

	SetAccoutInUse(address, passphrase string) bool
}

type EleWalletGUI interface {
	Init(langEN bool, backend GUIBackend)

	Show()

	SetLogger(log log.Logger)
}
