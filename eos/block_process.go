package eos

import (
	"context"
	"fmt"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/p2p"
	"github.com/eoscanada/eos-go/system"
	"github.com/eoscanada/eos-go/token"
	"log"
)

func (s *Server) BlockProcess(ctx context.Context, p2pAddr string, startBlock uint32) error {
	info, err := s.Api.GetInfo()
	if err != nil {
		log.Printf("block process: %s", err)
		return fmt.Errorf("block process: %s", err)
	}
	if startBlock == 0 {
		startBlock = info.HeadBlockNum
	}
	block, err := s.Api.GetBlockByNum(startBlock)
	if err != nil {
		log.Printf("block process: %s", err)
		return fmt.Errorf("block process: %s", err)
	}
	client := p2p.NewClient(p2pAddr, info.ChainID, 1206) // networkVersion from eos-go p2p-client

	handler := func(processable p2p.Message) {
		switch msg := processable.Envelope.P2PMessage.(type) {
		case *eos.SignedBlock:
			//log.Printf("got block: %d", msg.BlockNumber())
			for _, tx := range msg.Transactions {
				if tx.Transaction.Packed != nil {
					unpacked, err := tx.Transaction.Packed.Unpack()
					if err != nil {
						log.Printf("%s (block %d)", err, msg.BlockNumber())
						continue
					}
					for _, ac := range unpacked.Actions {
						go s.processAction(ac)
					}
					// TODO: parse context free actions (once it will exist)
				}
			}
		}
	}
	client.RegisterHandlerFunc(handler)
	err = client.ConnectAndSync(block.BlockNum, block.ID, block.Timestamp.Time, 0, make([]byte, 32))
	if err != nil {
		log.Println(err)
		return fmt.Errorf("block process: %s", err)
	}
	log.Println("started")
	<-ctx.Done()

	return ctx.Err()
}

func (s *Server) processAction(ac *eos.Action) {
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
			s.balanceChanged(op.From)
			s.balanceChanged(op.To)
		case *token.Issue:
			s.balanceChanged(op.To)
		// eosio
		case *system.BuyRAM:
			s.balanceChanged(op.Payer)
			s.balanceChanged(op.Receiver)
		case *system.BuyRAMBytes:
			s.balanceChanged(op.Payer)
			s.balanceChanged(op.Receiver)
		case *system.SellRAM:
			s.balanceChanged(op.Account)
		}
	}
}

func (s *Server) balanceChanged(account eos.AccountName) {
	if _, ok := s.tracked[string(account)]; ok {
		s.balanceChangedCh <- string(account)
	}
}
