package eosservice

import "github.com/Multy-io/Multy-back/store"

type Configuration struct {
	Name        string
	Account     string
	Key         string
	Host        string
	Port        string
	RPC         string
	P2P         string
	ServiceInfo store.ServiceInfo
}
