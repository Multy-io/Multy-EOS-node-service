package eos

import (
	"github.com/eoscanada/eos-go"
	"fmt"
	"encoding/json"
	"strconv"
	"context"

	pb "github.com/Multy-io/Multy-EOS-node-service/proto"
)

type RamMarket struct {
	Supply *eos.Asset `json:"supply"`
	Base  *BalanceWeight `json:"base"`
	Quote  *BalanceWeight `json:"quote"`
}

type BalanceWeight struct {
	Balance *eos.Asset `json:"balance"`
	Weight WeightType `json:"weight"`
}

type WeightType float64

func (w *WeightType) UnmarshalJSON(data []byte) error {
	var s string
	json.Unmarshal(data, &s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*w = WeightType(f)
	return nil
}

// getRAMPrice gets amount of RAM that you can buy for 1 EOS
func (s *Server) getRAMPrice(ctx context.Context, _ *pb.Empty) (*pb.RAMPrice, error) {
	rawResp, err := s.Api.GetTableRows(eos.GetTableRowsRequest{
		Code:  "eosio",
		Scope: "eosio",
		Table: "rammarket",
		JSON:  true,
	})
	if err != nil {
		return 0, err
	}
	resps := make([]*RamMarket, 1)
	err = rawResp.JSONToStructs(&resps)
	if err != nil {
		return 0, fmt.Errorf("unmarshall %s", err)
	}
	market := resps[0]

	price := (10000.0 - ((10000.0 + 199.0) / 200.0)) / (float64(market.Quote.Balance.Amount) / float64(market.Base.Balance.Amount))

	return &pb.RAMPrice{
		Price:price,
	}, nil
}
