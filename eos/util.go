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
		Amount:    a.Amount,
		Precision: uint32(a.Precision),
		Symbol:    a.Symbol.Symbol,
	}
}

// GetKey gets public key for given permission from account response structure
func GetKey(account *eos.AccountResp, permission string) (pubKey string) {
	for i := range account.Permissions {
		if account.Permissions[i].PermName == permission {
			// TODO: not sure what to return on multiple keys...
			if len(account.Permissions[i].RequiredAuth.Keys) != 1 {
				log.Errorf("account has multiple %s keys: %s", permission, account.AccountName)
				continue
			}
			pubKey = account.Permissions[i].RequiredAuth.Keys[0].PublicKey.String()
		}
	}
	return
}
