/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

// Package eos is a Multy node service gRPC server implementation
// See methods' descriptions if proto package
package eos

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
	"github.com/eoscanada/eos-go/p2p"
	"github.com/eoscanada/eos-go/system"
	// blank import for registering token actions.
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	_ "github.com/jekabolt/slflog"

	_ "github.com/eoscanada/eos-go/token"
)

const (
	// historyBufferSize is a size for users' history streaming
	historyBufferSize = 100
	// resyncTimeout is a timeout for account resync operation.
	// this is need to stop goroutines if something go wrong
	resyncTimeout = time.Hour * 12
)

// UserData is a multy wallet user data
type UserData struct {
	UserID       string
	WalletIndex  int32
	AddressIndex int32
}

// Server is a EOS node gRPC server struct
type Server struct {
	api     *eos.API
	p2pAddr string
	rpcAddr string

	account   eos.AccountName
	activeKey string

	version proto.ServiceVersion

	startBlockNum uint32 // TODO: pass this with requests

	// accounts to track
	trackedUsers map[string]UserData
	// user history chan
	historyCh chan proto.Action
}

// NewServer constructs new server.
// For proper usage you need to set version and signed
// using SetVersion & SetSigner
func NewServer(rpcAddr, p2pAddr string) *Server {
	server := &Server{
		api:           eos.New(rpcAddr),
		p2pAddr:       p2pAddr,
		rpcAddr:       rpcAddr,
		trackedUsers:  make(map[string]UserData),
		startBlockNum: 0, // 0 for most recent by default
		historyCh:     make(chan proto.Action, historyBufferSize),
	}
	return server
}

// SetVersion sets version info for multy-back to request
func (server *Server) SetVersion(branch, commit, buildtime, lasttag string) {
	server.version = proto.ServiceVersion{
		Branch:    branch,
		Buildtime: buildtime,
		Commit:    commit,
		Lasttag:   lasttag,
	}
	return
}

// SetSigner sets credentials for signer
func (server *Server) SetSigner(account, privKeyActive string) error {
	server.account = eos.AccountName(account)
	keyBag := eos.NewKeyBag()
	err := keyBag.ImportPrivateKey(privKeyActive)
	if err != nil {
		return err
	}
	server.api.SetSigner(keyBag)
	return nil
}

func (server *Server) ServiceInfo(_ context.Context, _ *proto.Empty) (*proto.ServiceVersion, error) {
	version := proto.ServiceVersion(server.version)
	return &version, nil
}

func (server *Server) InitialAdd(_ context.Context, userData *proto.UsersData) (*proto.ReplyInfo, error) {
	for key, val := range userData.GetMap() {
		// TODO: check if account exist?
		server.trackedUsers[key] = UserData{
			AddressIndex: val.AddressIndex,
			UserID:       val.UserID,
			WalletIndex:  val.WalletIndex,
		}
	}
	return &proto.ReplyInfo{}, nil
}

func (server *Server) AddNewAddress(_ context.Context, acc *proto.WatchAddress) (*proto.ReplyInfo, error) {
	// TODO: check if account exist?
	server.trackedUsers[acc.Address] = UserData{
		WalletIndex:  acc.WalletIndex,
		UserID:       acc.UserID,
		AddressIndex: acc.AddressIndex,
	}
	return &proto.ReplyInfo{}, nil
}

func (server *Server) GetBlockHeight(_ context.Context, _ *proto.Empty) (*proto.BlockHeight, error) {
	resp, err := server.api.GetInfo()
	if err != nil {
		return nil, err
	}
	return &proto.BlockHeight{
		HeadBlockNum: resp.HeadBlockNum,
		HeadBlockId:  hex.EncodeToString(resp.HeadBlockID),
	}, nil
}

func (server *Server) GetAddressBalance(_ context.Context, acc *proto.Account) (*proto.Balance, error) {
	resp, err := server.api.GetCurrencyBalance(eos.AN(acc.Name), "EOS", eos.AN("eosio.token"))
	if err != nil {
		return nil, err
	}
	if len(resp) != 1 {
		return nil, fmt.Errorf("EOS balance not single: %v", resp)
	}
	return &proto.Balance{
		Balance: resp[0].String(),
	}, nil
}

func (server *Server) resyncInternal(address string, user UserData, startBlockNum uint32) error {
	//TODO: consider streaming return
	// TODO: check if account exist?

	log.Debugf("resync %s", address)

	// check if account is in trackedUsers
	//userData, ok := server.trackedUsers[acc.Address]
	//if !ok {
	//	err := fmt.Errorf("user not trackedUsers: %s", acc.Address)
	//	return &proto.ReplyInfo{
	//		Message: err.Error(),
	//	}, err
	//}

	ctx := context.Background()
	singleTracker := map[string]UserData{address: user}
	handlerCtx, handlerCancel := context.WithTimeout(ctx, resyncTimeout)
	blockNumCh := make(chan uint32)

	handler := &blockDataHandler{
		blockNumCh:   blockNumCh,
		resync:       true,
		history:      server.historyCh,
		trackedUsers: singleTracker,
		name:         fmt.Sprintf("resync %s", address),
		ctx:          handlerCtx,
	}

	info, err := server.api.GetInfo()
	if err != nil {
		log.Errorf("%s get info %s", handler.name, err)
		return err
	}

	endBlockNum := info.HeadBlockNum

	block, err := server.api.GetBlockByNum(startBlockNum)
	if err != nil {
		log.Errorf("%s get info %s", handler.name, err)
		return err
	}

	p2pClient := p2p.NewClient(server.p2pAddr, info.ChainID, networkVersion)

	p2pClient.RegisterHandler(handler)
	go p2pClient.ConnectAndSync(block.BlockNum, block.ID, block.Timestamp.Time, 0, make([]byte, 32))

	go func() {
		defer p2pClient.UnregisterHandler(handler)
		var prevBlockNum, blockNum uint32
		prevBlockNum = startBlockNum
		for {
			select {
			case blockNum = <-blockNumCh:
				if blockNum-prevBlockNum > 10000 {
					// there is an issue when p2p client receives block way ahead of current state
					// e.g. when processing block 2000000 it receives block 8000000
					// this is workaround for this
					log.Errorf("%s got strange block num %d, previous %d", handler.name, blockNum, prevBlockNum)
					handlerCancel()
					server.resyncInternal(address, user, prevBlockNum)
					return
				}
				if blockNum > endBlockNum {
					log.Debugf("%s done %d", handler.name, blockNum)
					handlerCancel()
					return
				}
				if blockNum%1000 == 0 {
					log.Debugf("%s running, block %d", handler.name, blockNum)
				}
				prevBlockNum = blockNum
			case <-handlerCtx.Done():
				log.Errorf("done resync, err: %s, block: %d", handlerCtx.Err(), prevBlockNum)
				return
			}
		}
	}()

	err = ctx.Err()
	if err != nil {
		log.Errorf("%s cannot start %s", handler.name, err)
		p2pClient.UnregisterHandler(handler)
		return err
	}
	return err
}

func (server *Server) ResyncAddress(_ context.Context, acc *proto.AddressToResync) (*proto.ReplyInfo, error) {
	// TODO: consider streaming return
	// TODO: check if account exist?

	log.Debugf("ResyncAddress:resync %s", acc.Address)

	// check if account is in trackedUsers
	userData, ok := server.trackedUsers[acc.Address]
	if !ok {
		err := fmt.Errorf("user not trackedUsers: %s", acc.Address)
		return &proto.ReplyInfo{
			Message: err.Error(),
		}, err
	}

	account, err := server.api.GetAccount(eos.AN(acc.Address))
	if err != nil {
		log.Errorf("get account %s", err)
	}

	startBlockNum, err := server.GetBlockNumByTime(account.Created.Time)
	if err != nil {
		log.Errorf("resync %s estimate start block num: %s", acc.Address, err)
		startBlockNum = 1
	}
	if startBlockNum == 0 {
		startBlockNum = 1
	}

	err = server.resyncInternal(acc.Address, userData, startBlockNum)
	if err != nil {
		log.Errorf("resync internal %s", err)
		return &proto.ReplyInfo{
			Message: err.Error(),
		}, err
	}
	return &proto.ReplyInfo{}, nil
}

func (server *Server) NewBlock(_ *proto.Empty, stream proto.NodeCommunications_NewBlockServer) error {
	info, err := server.api.GetInfo()
	if err != nil {
		return fmt.Errorf("get_info: %s", err)
	}
	p2pClient := p2p.NewClient(server.p2pAddr, info.ChainID, networkVersion)
	heights := make(chan proto.BlockHeight)
	ctx := stream.Context()
	handlerCtx, handlerCancel := context.WithCancel(ctx)
	handler := &blockHeightHandler{
		ctx:         handlerCtx,
		blockHeight: heights,
	}
	p2pClient.RegisterHandler(handler)
	defer p2pClient.UnregisterHandler(handler)
	go p2pClient.ConnectRecent()

	for {
		select {
		case <-ctx.Done():
			handlerCancel()
			return ctx.Err()
		case height := <-heights:
			err = stream.Send(&height)
			if err != nil {
				handlerCancel()
				return err
			}
		}
	}
	return ctx.Err()

}

func (server *Server) SendRawTx(_ context.Context, rawTx *proto.RawTx) (*proto.SendTxResp, error) {
	tx := &eos.PackedTransaction{}
	err := json.Unmarshal(rawTx.Transaction, tx)
	if err != nil {
		return &proto.SendTxResp{}, err
	}
	resp, err := server.api.PushTransaction(tx)
	if err != nil {
		return &proto.SendTxResp{}, err
	}
	return &proto.SendTxResp{
		TransactionId: resp.TransactionID,
	}, nil
}

func (server *Server) NewTx(_ *proto.Empty, stream proto.NodeCommunications_NewTxServer) error {
	info, err := server.api.GetInfo()
	if err != nil {
		return fmt.Errorf("get_info: %s", err)
	}

	startBlockNum := server.startBlockNum
	if startBlockNum == 0 {
		startBlockNum = info.HeadBlockNum
	}
	startBlock, err := server.api.GetBlockByNum(startBlockNum)
	if err != nil {
		return fmt.Errorf("get_block: %s", err)
	}

	ctx := stream.Context()
	handlerCtx, handlerCancel := context.WithCancel(ctx)

	handler := &blockDataHandler{
		ctx:          handlerCtx,
		name:         "NewTx",
		trackedUsers: server.trackedUsers,
		history:      server.historyCh,
		resync:       false,
	}

	p2pClient := p2p.NewClient(server.p2pAddr, info.ChainID, networkVersion)
	p2pClient.RegisterHandler(handler)
	defer p2pClient.UnregisterHandler(handler)
	go p2pClient.ConnectAndSync(startBlockNum, startBlock.ID, startBlock.Timestamp.Time, 0, make([]byte, 32))

	for {
		select {
		case action := <-server.historyCh:
			err = stream.Send(&action)
			if err != nil {
				handlerCancel()
				return err
			}
		case <-ctx.Done():
			handlerCancel()
			return ctx.Err()
		}
	}
	return ctx.Err()
}

func (server *Server) SyncState(_ context.Context, height *proto.BlockHeight) (*proto.ReplyInfo, error) {
	server.startBlockNum = height.HeadBlockNum
	return &proto.ReplyInfo{}, nil
}

func (server *Server) AccountCreate(ctx context.Context, req *proto.AccountCreateReq) (*proto.ReplyInfo, error) {
	ownerKey, err := ecc.NewPublicKey(req.OwnerKey)
	if err != nil {
		err = fmt.Errorf("owner: %s", err)
		return &proto.ReplyInfo{
			Message: err.Error(),
		}, err
	}
	activeKey, err := ecc.NewPublicKey(req.OwnerKey)
	if err != nil {
		err = fmt.Errorf("active: %s", err)
		return &proto.ReplyInfo{
			Message: err.Error(),
		}, err
	}
	newAcc := system.NewCustomNewAccount(
		server.account,
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
	buyRAM := system.NewBuyRAM(server.account, eos.AccountName(req.Name), req.Ram)

	delegateBW := system.NewDelegateBW(server.account, eos.AN(req.Name),
		eos.NewEOSAsset(req.Cpu), eos.NewEOSAsset(req.Net), true)

	_, err = server.api.SignPushActions(newAcc, buyRAM, delegateBW)
	if err != nil {
		err = fmt.Errorf("push tx: %s", err)
		return &proto.ReplyInfo{
			Message: err.Error(),
		}, err
	}
	return &proto.ReplyInfo{}, nil
}

func (server *Server) AccountCheck(ctx context.Context, req *proto.Account) (*proto.AccountInfo, error) {
	account, err := server.api.GetAccount(eos.AN(req.Name))
	// TODO: check for errors?
	return &proto.AccountInfo{
		Exist:     err == nil,
		PublicKey: GetKey(account, "owner"),
		ActiveKey: GetKey(account, "active"),
		OwnerKey:  GetKey(account, "owner"),
	}, nil
}

func (server *Server) GetTokenBalance(ctx context.Context, req *proto.BalanceReq) (*proto.Balances, error) {
	code := req.Code
	if code == "" {
		code = "eosio.token"
	}
	resp, err := server.api.GetCurrencyBalance(eos.AccountName(req.Account), req.Symbol, eos.AccountName(code))
	if err != nil {
		return nil, err
	}
	balances := &proto.Balances{
		Assets: make([]*proto.Asset, len(resp)),
	}
	for i, a := range resp {
		balances.Assets[i] = asset(a)
	}
	return balances, nil
}

func (server *Server) GetChainState(_ context.Context, _ *proto.Empty) (*proto.ChainState, error) {
	resp, err := server.api.GetInfo()
	if err != nil {
		return nil, err
	}
	return &proto.ChainState{
		HeadBlockNum:             resp.HeadBlockNum,
		HeadBlockId:              resp.HeadBlockID,
		HeadBlockTime:            resp.HeadBlockTime.Unix(),
		LastIrreversibleBlockNum: resp.LastIrreversibleBlockNum,
		LastIrreversibleBlockId:  resp.LastIrreversibleBlockID,
	}, nil
}

func (server *Server) GetKeyAccounts(_ context.Context, req *proto.PublicKey) (*proto.Accounts, error) {
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(fmt.Sprintf("%s/v1/history/get_key_accounts", server.rpcAddr),
		"application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// TODO more sane response, needs node research
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("response not ok: %v", string(bs))
	}

	var accounts proto.Accounts
	err = json.NewDecoder(resp.Body).Decode(&accounts)

	ownerAccounts := make([]string, 0, len(accounts.AccountNames))
	for _, name := range accounts.AccountNames {
		accountResp, err := server.api.GetAccount(eos.AN(name))
		if err != nil {
			log.Errorf("get account: %s", err)
		}
		if req.PublicKey == GetKey(accountResp, "owner") {
			ownerAccounts = append(ownerAccounts, name)
		}
	}
	accounts.AccountNames = ownerAccounts
	return &accounts, err
}
