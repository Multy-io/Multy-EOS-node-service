/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"context"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/p2p"
	"github.com/eoscanada/eos-go/system"
	"github.com/eoscanada/eos-go/token"
	"log"

	"encoding/hex"
	"github.com/Multy-io/Multy-EOS-node-service/proto"
)

var networkVersion = uint16(1206) // networkVersion from eos-go p2p-client tool

// ProcessValue type is used for passing value to context
// this is done to avoid collisions
// (context docs tells you to do this)
type ProcessValue string

type blockDataHandler struct {

	// context docs tells you that storing context in struct is bad
	// but it's the only way
	ctx context.Context

	name         string
	history      chan proto.EOSAction
	resync       bool
	trackedUsers map[string]UserData

	//startBlockNum uint32
	//endBlockNum uint32

	blockNumCh chan uint32
}

func (handler blockDataHandler) Handle(msg p2p.Message) {
	select {
	case <-handler.ctx.Done():
		return
	default:
		if msg.Envelope.Type == eos.SignedBlockType {
			block := msg.Envelope.P2PMessage.(*eos.SignedBlock)
			if num := block.BlockNumber(); num%10000 == 0 {
				log.Printf("process block %d", block.BlockNumber())
			}
			if handler.blockNumCh != nil {
				handler.blockNumCh <- block.BlockNumber()
			}
			for txNum := range block.Transactions {
				tx := &block.Transactions[txNum]
				if tx.Transaction.Packed != nil {
					unpacked, err := tx.Transaction.Packed.Unpack()
					if err != nil {
						log.Printf("%s (block %d, %s)", err, block.BlockNumber(), handler.name)
						continue
					}
					for _, ac := range unpacked.Actions {
						go handler.processAction(handler.ctx, ac)
					}
					// TODO: parse context free actions (once it will exist)
				}
			}
		}

	}
}

func (handler *blockDataHandler) processAction(ctx context.Context, ac *eos.Action) {
	if ac.Data != nil {
		err := ac.MapToRegisteredAction()
		if err != nil {
			log.Println(err)
			return
		}

		// check for default smart-contracts' action
		switch op := ac.Data.(type) {
		// eosio.token
		case *token.Transfer:
			toSend := proto.EOSAction{
				Type:   proto.EOSAction_TRANSFER_TOKEN,
				From:   string(op.From),
				To:     string(op.To),
				Amount: asset(op.Quantity),
				Memo:   op.Memo,
			}
			handler.sendHistory(ctx, toSend, op.From)
			handler.sendHistory(ctx, toSend, op.To)
		case *token.Issue:
			toSend := proto.EOSAction{
				Type:   proto.EOSAction_ISSUE_TOKEN,
				From:   "eosio.token", // this is default token contract
				To:     string(op.To),
				Amount: asset(op.Quantity),
				Memo:   op.Memo,
			}

			handler.sendHistory(ctx, toSend, op.To)
		// eosio
		case *system.BuyRAM:
			toSend := proto.EOSAction{
				Type:   proto.EOSAction_BUY_RAM,
				From:   string(op.Payer),
				To:     string(op.Receiver),
				Amount: asset(op.Quantity),
			}
			handler.sendHistory(ctx, toSend, op.Payer)
			handler.sendHistory(ctx, toSend, op.Receiver)
		case *system.BuyRAMBytes:
			toSend := proto.EOSAction{
				Type:   proto.EOSAction_BUY_RAM_BYTES,
				From:   string(op.Payer),
				To:     string(op.Receiver),
				Amount: makeRAM(uint64(op.Bytes)),
			}
			handler.sendHistory(ctx, toSend, op.Payer)
			handler.sendHistory(ctx, toSend, op.Receiver)
		case *system.SellRAM:
			toSend := proto.EOSAction{
				Type:   proto.EOSAction_SELL_RAM,
				From:   string(op.Account),
				To:     string(op.Account), // you sell it for yourself
				Amount: makeRAM(op.Bytes),
			}
			handler.sendHistory(ctx, toSend, op.Account)
		}
	}
}

// sendHistory checks if user is trackedUsers
// and fills user data fields
// and then sends extended action data to a chanel
func (handler *blockDataHandler) sendHistory(ctx context.Context, action proto.EOSAction, account eos.AccountName) {
	if user, ok := handler.trackedUsers[string(account)]; ok {
		log.Printf("found action %s", account)
		action.Resync = handler.resync

		action.UserID = user.UserID
		action.WalletIndex = user.WalletIndex
		action.AddressIndex = user.AddressIndex
		select {
		case <-ctx.Done():
			return
		case handler.history <- action:
			return
		}
	}
}

// makeRAM makes asset of RAM
func makeRAM(bytes uint64) *proto.Asset {
	return &proto.Asset{
		Symbol:    "RAM",
		Ammount:   int64(bytes),
		Precision: 0,
	}
}

type blockHeightHandler struct {
	ctx context.Context

	blockHeight chan proto.BlockHeight
}

func (handler blockHeightHandler) Handle(processable p2p.Message) {
	select {
	case <-handler.ctx.Done():
		return
	default:
		log.Println(processable.Envelope.Type.Name())
		if processable.Envelope.Type == eos.SignedBlockType {
			block := processable.Envelope.P2PMessage.(*eos.SignedBlock)
			id, err := block.BlockID()
			if err != nil {
				log.Printf("block_id: %s", err)
				return
			}
			log.Println("handler send")
			handler.blockHeight <- proto.BlockHeight{
				HeadBlockNum: block.BlockNumber(),
				HeadBlockId:  hex.EncodeToString(id),
			}
			log.Println("handler send done")
		}
	}
}

//// ProcessBlockHeight fetches new block data
//func (server *Server) ProcessBlockHeight(ctx context.Context, name string) error {
//	log.Println("ProcessBlockHeight")
//	info, err := server.api.GetInfo()
//	if err != nil {
//		log.Printf("block process: %s (%s)", err, name)
//		return fmt.Errorf("block process : %s", err, name)
//	}
//	handlerCtx, handlerCancel := context.WithCancel(ctx)
//	handler := p2p.HandlerFunc(func(processable p2p.Message) {
//		select {
//		case <-handlerCtx.Done():
//			return
//		default:
//			log.Println("type")
//			log.Println(processable.Envelope.Type.Name())
//			if processable.Envelope.Type == eos.SignedBlockType {
//				msg := processable.Envelope.P2PMessage.(*eos.SignedBlock)
//				id, err := msg.BlockID()
//				if err != nil {
//					log.Printf("block_id: %s", err)
//					return
//				}
//				server.blockHeightCh <- proto.BlockHeight{
//					HeadBlockNum: msg.BlockNumber(),
//					HeadBlockId:  hex.EncodeToString(id),
//				}
//			}
//		}
//	})
//	client := p2p.NewClient(server.p2pAddr, info.ChainID, networkVersion)
//	client.RegisterHandler(handler)
//	err = client.ConnectRecent()
//	if err != nil {
//		log.Println(err)
//		handlerCancel()
//		// BUG: UnregisterHandler of HandlerFunc panics
//		//client.UnregisterHandler(handler)
//		return err
//	}
//	for {
//		select {
//		case <-ctx.Done():
//			handlerCancel()
//			break
//		default:
//			time.Sleep(time.Millisecond * 100) // 0.1 second
//		}
//	}
//	// BUG: UnregisterHandler of HandlerFunc panics
//	//client.UnregisterHandler(handler)
//	return nil
//}
