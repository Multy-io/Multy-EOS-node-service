/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package main

import (
	"context"
	"fmt"
	"net"

	"github.com/Multy-io/Multy-EOS-node-service"
	"github.com/Multy-io/Multy-EOS-node-service/eos"
	pb "github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/Multy-io/Multy-back/store"
	"github.com/jekabolt/config"
	"github.com/jekabolt/slf"
	_ "github.com/jekabolt/slflog"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

var (
	commit    string
	branch    string
	buildtime string
	lasttag   string
	log       = slf.WithContext("main")
)

var globalOpt = eosservice.Configuration{
	Name: "eos-service",
}

func main() {
	config.ReadGlobalConfig(&globalOpt, "eos-service configuration")
	log.Infof("CONFIGURATION=%+v", globalOpt)
	log.Infof("branch: %s", branch)
	log.Infof("commit: %s", commit)
	log.Infof("build time: %s", buildtime)
	globalOpt.ServiceInfo = store.ServiceInfo{
		Branch:    branch,
		Commit:    commit,
		Buildtime: buildtime,
	}
	initService(globalOpt)

	block := make(chan bool)
	<-block
}

func initService(conf eosservice.Configuration) error {
	server := eos.NewServer(
		conf.RPC,
		conf.P2P,
	)
	err := server.SetSigner(conf.Account, conf.Key)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("cannot init server: %s", err), 2)
	}
	server.SetVersion(branch, commit, buildtime, lasttag)
	log.Infof("new server")

	server.GetChainState(context.Background(), &pb.Empty{})

	addr := fmt.Sprintf("%s:%s", conf.Host, conf.Port)

	// init gRPC server
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to listen: %s", err), 2)
	}
	// Creates a new gRPC server
	s := grpc.NewServer()
	pb.RegisterNodeCommunicationsServer(s, server)

	log.Infof("listening on %s", addr)
	err = s.Serve(lis)
	return cli.NewExitError(err, 3)
}
