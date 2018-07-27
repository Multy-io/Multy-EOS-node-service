/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"context"
	"fmt"
	"github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/eoscanada/eos-go"
)

type ramMarket struct {
	Supply *eos.Asset     `json:"supply"`
	Base   *balanceWeight `json:"base"`
	Quote  *balanceWeight `json:"quote"`
}

type balanceWeight struct {
	Balance *eos.Asset `json:"balance"`
	//weight is omitted, we don't need it here
}

// GetRAMPrice gets amount of RAM that you can buy for 1 EOS
func (server *Server) GetRAMPrice(ctx context.Context, _ *proto.Empty) (*proto.RAMPrice, error) {
	rawResp, err := server.api.GetTableRows(eos.GetTableRowsRequest{
		Code:  "eosio",
		Scope: "eosio",
		Table: "rammarket",
		JSON:  true,
	})
	if err != nil {
		return &proto.RAMPrice{
			Price: 0,
		}, err
	}
	markets := make([]*ramMarket, 1)
	err = rawResp.JSONToStructs(&markets)
	if err != nil {
		return &proto.RAMPrice{
			Price: 0,
		}, fmt.Errorf("unmarshall %s", err)
	}
	market := markets[0]

	// 0.5% fee, from eos source
	// 10000 == 1 EOS without precision
	price := (10000.0 - ((10000.0 + 199.0) / 200.0)) / (float64(market.Quote.Balance.Amount) / float64(market.Base.Balance.Amount))

	return &proto.RAMPrice{
		Price: price,
	}, nil
}
