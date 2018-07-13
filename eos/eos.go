/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"context"
	"encoding/hex"
	"fmt"
	pb "github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/eoscanada/eos-go/system"
	_ "github.com/eoscanada/eos-go/token"
)

const MESSAGE_BUFFER_SIZE = 100

type Server struct {
	Api       *eos.API
	account   eos.AccountName
	activeKey string

	tracked          map[string]bool // accounts to track
	balanceChangedCh chan string
}

func NewServer(rpcAddr, p2pAddr, account, privKey string, startBlock uint32) (*Server, error) {
	client := &Server{
		Api:              eos.New(rpcAddr),
		account:          eos.AccountName(account),
		activeKey:        privKey,
		tracked:          make(map[string]bool),
		balanceChangedCh: make(chan string, MESSAGE_BUFFER_SIZE),
	}
	keyBag := eos.NewKeyBag()
	err := keyBag.ImportPrivateKey(privKey)
	if err != nil {
		return nil, err
	}
	client.Api.SetSigner(keyBag)
	go client.BlockProcess(context.TODO(), p2pAddr, startBlock)

	return client, nil
}

func (c *Server) EventGetChainInfo(ctx context.Context, _ *pb.Empty) (*pb.ChainInfo, error) {
	resp, err := c.Api.GetInfo()
	if err != nil {
		return nil, err
	}
	return &pb.ChainInfo{
		HeadBlockNum:             resp.HeadBlockNum,
		HeadBlockId:              hex.EncodeToString(resp.HeadBlockID),
		HeadBlockTime:            resp.HeadBlockTime.Unix(),
		LastIrreversibleBlockNum: resp.LastIrreversibleBlockNum,
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
		Assets: make([]*pb.Asset, len(resp)),
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
		Name:              string(resp.AccountName),
		CoreLiquidBalance: asset(resp.CoreLiquidBalance),
		RamAvailable:      resp.RAMQuota - resp.RAMUsage,
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
		Signatures:            signatures,
		Compression:           eos.CompressionType(req.Compression),
		PackedContextFreeData: req.PackedContextFreeData,
		PackedTransaction:     req.PackedTrx,
	}
	resp, err := c.Api.PushTransaction(&tx)
	if err != nil {
		return &pb.PushTransactionResp{}, nil
	}
	return &pb.PushTransactionResp{
		Id:         resp.TransactionID,
		StatusCode: resp.StatusCode,
	}, nil
}

func (c *Server) EventGetTransactionInfo(ctx context.Context, req *pb.TransactionID) (*pb.TransactionInfo, error) {
	resp, err := c.Api.GetTransaction(req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.TransactionInfo{
		Id:       hex.EncodeToString(resp.ID),
		BlockNum: resp.BlockNum,
	}, nil
}

func (c *Server) EventAccountCreate(ctx context.Context, req *pb.AccountCreateReq) (*pb.OkErrResponse, error) {
	ownerKey, err := ecc.NewPublicKey(req.OwnerKey)
	if err != nil {
		return &pb.OkErrResponse{
			Ok:    false,
			Error: fmt.Sprintf("create account: owner key: %s", err),
		}, nil
	}
	activeKey, err := ecc.NewPublicKey(req.OwnerKey)
	if err != nil {
		return &pb.OkErrResponse{
			Ok:    false,
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
					Weight:    1,
				},
			},
		},
		eos.Authority{
			Threshold: 1,
			Keys: []eos.KeyWeight{
				{
					PublicKey: activeKey,
					Weight:    1,
				},
			},
		},
	)
	buyRAM := system.NewBuyRAM(c.account, eos.AccountName(req.Name), req.RamCost)

	// TODO: cleos does delegatebw on newaccount

	_, err = c.Api.SignPushActions(newAcc, buyRAM)
	if err != nil {
		return &pb.OkErrResponse{
			Ok:    false,
			Error: fmt.Sprintf("create account: push tx: %s", err),
		}, nil
	}
	return &pb.OkErrResponse{
		Ok: true,
	}, nil
}

func (s *Server) EventAccountCheck(ctx context.Context, req *pb.Account) (*pb.Exist, error) {
	_, err := s.Api.GetAccount(eos.AN(req.Name))
	return &pb.Exist{
		Exist: err == nil,
	}, nil
}

func (s *Server) BalanceChanged(_ *pb.Empty, stream pb.NodeCommunications_BalanceChangedServer) error {
	for {
		select {
		case account := <-s.balanceChangedCh:
			balResp, err := s.EventGetBalance(context.TODO(), &pb.BalanceReq{Name: account})
			if err != nil {
				return err
			}
			ramResp, err := s.EventGetAccount(context.TODO(), &pb.Account{Name: account})
			balResp.Assets = append(balResp.Assets, &pb.Asset{Precision: 0, Symbol: "RAM", Ammount: ramResp.RamAvailable})
			stream.Send(balResp)
		}
	}
	return nil
}

func (s *Server) EventTrackAccount(ctx context.Context, req *pb.Account) (*pb.Empty, error) {
	s.tracked[req.Name] = true
	return &pb.Empty{}, nil
}

func (s *Server) EventSetTrackedAccounts(ctx context.Context, req *pb.Accounts) (*pb.Empty, error) {
	new := make(map[string]bool)
	for _, acc := range req.Accounts {
		new[acc.Name] = true
	}
	s.tracked = new
	return &pb.Empty{}, nil

	return &pb.Empty{}, nil
}
