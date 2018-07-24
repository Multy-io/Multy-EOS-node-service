/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/eoscanada/eos-go"
)

// asset constructs protobuf asset struct
// from eos-go asset struct
func asset(a eos.Asset) *proto.Asset {
	return &proto.Asset{
		Ammount:   a.Amount,
		Precision: uint32(a.Precision),
		Symbol:    a.Symbol.Symbol,
	}
}
