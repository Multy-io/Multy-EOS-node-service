/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package eos

import (
	"github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
)

func signatures(protoSignatures []string) ([]ecc.Signature, error) {
	res := make([]ecc.Signature, 0, len(protoSignatures))
	for _, str := range protoSignatures {
		sig, err := ecc.NewSignature(str)
		if err != nil {
			return res, err
		}
		res = append(res, sig)
	}
	return res, nil
}
func compressionType(protoType proto.RawTx_Compression) eos.CompressionType {
	switch protoType {
	case proto.RawTx_NONE:
		return eos.CompressionNone
	case proto.RawTx_ZLIB:
		return eos.CompressionZlib
	}
	return eos.CompressionNone
}

// asset constructs protobuf asset struct
// from eos-go asset struct
func asset(a eos.Asset) *proto.Asset {
	return &proto.Asset{
		Ammount:   a.Amount,
		Precision: uint32(a.Precision),
		Symbol:    a.Symbol.Symbol,
	}
}
