// Copyright 2018 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

package node

import (
	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/ethdb"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/p2p"
	"github.com/contatract/go-contatract/rpc"
	"reflect"
)

// ServiceContext is a collection of service independent options inherited from
// the protocol stack, that is passed to all constructors to be optionally used;
// as well as utility methods to operate on the service environment.
type FTransServiceContext struct {
	Config         *NodeConfig
	Services       map[reflect.Type]FTransService // Index of the already constructed services
	EventMux       *event.TypeMux                 // Event multiplexer used for decoupled notifications
	AccountManager *accounts.Manager              // Account manager created by the node.
}

func (s *FTransServiceContext) SetService(kind reflect.Type, srv FTransService) {
	s.Services[kind] = srv
}

// OpenDatabase opens an existing database with the given name (or creates one
// if no previous can be found) from within the node's data directory. If the
// node is an ephemeral one, a memory database is returned.
func (ctx *FTransServiceContext) OpenDatabase(name string, cache int, handles int) (ethdb.Database, error) {
	if ctx.Config.DataDir == "" {
		return ethdb.NewMemDatabase()
	}
	db, err := ethdb.NewLDBDatabase(ctx.Config.resolvePath(name), cache, handles)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// ResolvePath resolves a user path into the data directory if that was relative
// and if the user actually uses persistent storage. It will return an empty string
// for emphemeral storage and the user's own input for absolute paths.
func (ctx *FTransServiceContext) ResolvePath(path string) string {
	return ctx.Config.resolvePath(path)
}

// Service retrieves a currently running service registered of a specific type.
func (ctx *FTransServiceContext) Service(service interface{}) error {
	element := reflect.ValueOf(service).Elem()
	if running, ok := ctx.Services[element.Type()]; ok {
		element.Set(reflect.ValueOf(running))
		return nil
	}
	return ErrServiceUnknown
}

// ServiceConstructor is the function signature of the constructors needed to be
// registered for service instantiation.
type FTransServiceConstructor func(ctx *FTransServiceContext) (FTransService, error)

type FTransService interface {
	// Protocols retrieves the P2P protocols the service wishes to start.
	Protocols() []p2p.Protocol

	// APIs retrieves the list of RPC descriptors the service provides
	APIs() []rpc.API

	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	StartClient(*p2p.Client) error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop() error
}
