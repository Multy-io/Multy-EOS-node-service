/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/system"
	"fmt"
	"github.com/eoscanada/eos-go/ecc"
	pb "github.com/Multy-io/Multy-EOS-node-service/proto"
	"context"
	"encoding/hex"
)

type Server struct {
	Api           *eos.API
	account       eos.AccountName
	activeKey string
}

func NewServer(url, account, privKey string) (*Server, error) {
	client := &Server{
		Api:           eos.New(url),
		account:       eos.AccountName(account),
		activeKey: privKey,
	}
	keyBag := eos.NewKeyBag()
	err := keyBag.ImportPrivateKey(privKey)
	if err != nil {
		return nil, err
	}
	client.Api.SetSigner(keyBag)
	return client, nil
}

func (c *Server) EventGetChainInfo (ctx context.Context, _ *pb.Empty) (*pb.ChainInfo, error) {
	resp, err := c.Api.GetInfo()
	if err != nil {
		return nil, err
	}
	return &pb.ChainInfo{
		HeadBlockNum:resp.HeadBlockNum,
		HeadBlockId: hex.EncodeToString(resp.HeadBlockID),
		HeadBlockTime:resp.HeadBlockTime.Unix(),
		LastIrreversibleBlockNum:resp.LastIrreversibleBlockNum,
	}, nil
}

// EventGetBalance only gets 'eosio.token' balance for now
func (c *Server) EventGetBalance(ctx context.Context, req *pb.BalanceReq) (*pb.Balances, error) {
	code := req.Code
	if code == "" {
		code = "eosio.token"
	}
	resp, err := c.Api.GetCurrencyBalance(eos.AccountName(req.Name), req.Symbol, eos.AccountName(code))
	//resp, err := c.Api.GetAccount(eos.AccountName(req.Name))
	if err != nil {
		return nil, err
	}
	balances := &pb.Balances{
		Assets:make([]*pb.Asset, len(resp)),
	}
	for i, a := range resp {
		balances.Assets[i] = asset(a)
	}
	return balances, nil
}

func asset(a eos.Asset) *pb.Asset {
	return &pb.Asset{
		Ammount:   a.Amount,
		Precision: uint32(a.Precision),
		Symbol:    a.Symbol.Symbol,
	}
}

func (c *Server) EventGetAccount(ctx context.Context, req *pb.Account) (*pb.AccountInfo, error) {
	resp, err := c.Api.GetAccount(eos.AccountName(req.Name))
	if err != nil {
		return nil, err
	}
	return &pb.AccountInfo{
		Name:string(resp.AccountName),
		CoreLiquidBalance: asset(resp.CoreLiquidBalance),
		RamAvailable: resp.RAMQuota - resp.RAMUsage,
	}, nil
}

func (c *Server) EventPushTransaction(ctx context.Context, req *pb.Transaction) (*pb.PushTransactionResp, error) {
	signatures := make([]ecc.Signature, 0, len(req.Signatures))
	for _, sig := range req.Signatures {
		txSig, err := ecc.NewSignature(sig)
		if err != nil {
			return nil, err
		}
		signatures = append(signatures, txSig)
	}
	tx := eos.PackedTransaction{
		Signatures: signatures,
		Compression:eos.CompressionType(req.Compression),
		PackedContextFreeData:req.PackedContextFreeData,
		PackedTransaction:req.PackedTrx,
	}
	resp, err := c.Api.PushTransaction(&tx)
	if err != nil {
		return &pb.PushTransactionResp{
		}, nil
	}
	return &pb.PushTransactionResp{
		Id:resp.TransactionID,
		StatusCode:resp.StatusCode,
	}, nil
}

func (c *Server) EventGetTransactionInfo(ctx context.Context, req *pb.TransactionID) (*pb.TransactionInfo, error) {
	resp, err := c.Api.GetTransaction(req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.TransactionInfo{
		Id:hex.EncodeToString(resp.ID),
		BlockNum:resp.BlockNum,
	}, nil
}

func (c *Server) EventAccountCreate(ctx context.Context, req *pb.AccountCreateReq) (*pb.OkErrResponse, error){
	ownerKey, err := ecc.NewPublicKey(req.OwnerKey)
	if err != nil {
		return &pb.OkErrResponse{
			Ok: false,
			Error: fmt.Sprintf("create account: owner key: %s", err),
		}, nil
	}
	activeKey, err := ecc.NewPublicKey(req.OwnerKey)
	if err != nil {
		return &pb.OkErrResponse{
			Ok: false,
			Error: fmt.Sprintf("create account: active key: %s", err),
		}, nil
	}

	newAcc := system.NewCustomNewAccount(
		c.account,
		eos.AccountName(req.Name),
		eos.Authority{
			Threshold: 1,
			Keys: []eos.KeyWeight{
				{
					PublicKey: ownerKey,
					Weight: 1,
				},
			},
		},
		eos.Authority{
			Threshold: 1,
			Keys: []eos.KeyWeight{
				{
					PublicKey: activeKey,
					Weight: 1,
				},
			},
		},
	)
	buyRAM := system.NewBuyRAM(c.account, eos.AccountName(req.Name), req.RamCost)

	// TODO: cleos does delegatebw on newaccount

	_, err = c.Api.SignPushActions(newAcc, buyRAM)
	if err != nil {
		return &pb.OkErrResponse{
			Ok:false,
			Error:fmt.Sprintf("create account: push tx: %s", err),
		}, nil
	}
	return &pb.OkErrResponse{
		Ok:true,
	}, nil
}

