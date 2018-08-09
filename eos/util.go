/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/eoscanada/eos-go"
	"github.com/pkg/errors"
	"time"
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

// GetBlockNumByTime estimates block number that has time before the given time
func (server *Server) GetBlockNumByTime(blockTime time.Time) (uint32, error) {
	info, err := server.api.GetInfo()
	if err != nil {
		return 0, err
	}
	startBlock, err := server.api.GetBlockByNum(1)
	if err != nil {
		return 0, err
	}
	guess := uint32(float64(info.HeadBlockNum) / info.HeadBlockTime.Sub(startBlock.Timestamp.Time).Seconds() * blockTime.Sub(startBlock.Timestamp.Time).Seconds())
	for {
		if guess > info.HeadBlockNum {
			break
		}
		block, err := server.api.GetBlockByNum(guess)
		if err != nil {
			return 0, err
		}
		if block.Timestamp.Before(blockTime) {
			return block.BlockNum, nil
		}
		guess -= uint32(time.Hour.Seconds()) * 2

	}
	return 0, errors.New("something went wrong")
}
