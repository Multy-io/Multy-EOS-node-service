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

type blockDataHandler struct {

	// context docs tells you that storing context in struct is bad
	// but it's the only way
	ctx context.Context

	name         string
	history      chan proto.Action
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
					for idx, action := range unpacked.Actions {
						go handler.processAction(action, tx.Transaction.ID, int64(idx))
					}
					// TODO: parse context free actions (once it will exist)
				}
			}
		}

	}
}

func (handler *blockDataHandler) processAction(action *eos.Action, transactionID eos.SHA256Bytes, actionIndex int64) {
	if action.Data != nil {
		err := action.MapToRegisteredAction()
		if err != nil {
			log.Println(err)
			return
		}

		toSend := proto.Action{
			ActionIndex:   actionIndex,
			TransactionId: transactionID,
		}

		// check for default smart-contracts' action
		switch op := action.Data.(type) {
		// eosio.token
		case *token.Transfer:
			toSend.Type = proto.Action_TRANSFER_TOKEN
			toSend.From = string(op.From)
			toSend.To = string(op.To)
			toSend.Amount = asset(op.Quantity)
			toSend.Memo = op.Memo

			handler.sendHistory(toSend, op.From)
			handler.sendHistory(toSend, op.To)
		case *token.Issue:
			toSend.Type = proto.Action_ISSUE_TOKEN
			toSend.From = "eosio.token" // this is default token contract
			toSend.To = string(op.To)
			toSend.Amount = asset(op.Quantity)
			toSend.Memo = op.Memo

			handler.sendHistory(toSend, op.To)
		// eosio
		case *system.BuyRAM:
			toSend.Type = proto.Action_BUY_RAM
			toSend.From = string(op.Payer)
			toSend.To = string(op.Receiver)
			toSend.Amount = asset(op.Quantity)

			handler.sendHistory(toSend, op.Payer)
			handler.sendHistory(toSend, op.Receiver)
		case *system.BuyRAMBytes:
			toSend.Type = proto.Action_BUY_RAM_BYTES
			toSend.From = string(op.Payer)
			toSend.To = string(op.Receiver)
			toSend.Amount = makeRAM(uint64(op.Bytes))

			handler.sendHistory(toSend, op.Payer)
			handler.sendHistory(toSend, op.Receiver)
		case *system.SellRAM:
			toSend.From = string(op.Account)
			toSend.To = string(op.Account) // you sell it for yourself
			toSend.Amount = makeRAM(op.Bytes)

			handler.sendHistory(toSend, op.Account)
		}
	}
}

// sendHistory checks if user is trackedUsers
// and fills user data fields
// and then sends extended action data to a chanel
func (handler *blockDataHandler) sendHistory(action proto.Action, account eos.AccountName) {
	if user, ok := handler.trackedUsers[string(account)]; ok {
		log.Printf("found action %s", account)
		action.Resync = handler.resync

		action.UserID = user.UserID
		action.WalletIndex = user.WalletIndex
		action.AddressIndex = user.AddressIndex
		action.Address = string(account)
		select {
		case <-handler.ctx.Done():
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
		Amount:    int64(bytes),
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
