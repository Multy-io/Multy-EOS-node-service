/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

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
