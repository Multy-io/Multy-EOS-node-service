/*
 * Copyright 2018 Idealnaya rabota LLC
 * Licensed under Multy.io license.
 * See LICENSE for details
 */

package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/Multy-io/Multy-EOS-node-service/eos"
	pb "github.com/Multy-io/Multy-EOS-node-service/proto"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

var (
	commit    string
	branch    string
	buildtime string
	lasttag   string
)

func run(c *cli.Context) error {
	server := eos.NewServer(
		c.String("rpc"),
		c.String("p2p"),
	)
	err := server.SetSigner(c.String("account"), c.String("key"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("cannot init server: %s", err), 2)
	}
	server.SetVersion(branch, commit, buildtime, lasttag)
	log.Println("new server")

	addr := fmt.Sprintf("%s:%s", c.String("host"), c.String("port"))

	// init gRPC server
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("failed to listen: %s", err), 2)
	}
	// Creates a new gRPC server
	s := grpc.NewServer()
	pb.RegisterNodeCommunicationsServer(s, server)

	log.Printf("listening on %s", addr)
	err = s.Serve(lis)
	return cli.NewExitError(err, 3)
}

func main() {
	app := cli.NewApp()
	app.Name = "multy-eos"
	app.Usage = `eos node gRPC API for Multy backend`
	app.Version = fmt.Sprintf("%s (commit: %s, branch: %s, buildtime: %s)", lasttag, commit, branch, buildtime)
	app.Author = "vovapi"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "host",
			Usage:  "hostname to bind to",
			EnvVar: "MULTY_EOS_HOST",
			Value:  "",
		},
		cli.StringFlag{
			Name:   "port",
			Usage:  "port to bind to",
			EnvVar: "MULTY_EOS_PORT",
			Value:  "8080",
		},
		cli.StringFlag{
			Name:   "rpc",
			Usage:  "node rpc api address",
			EnvVar: "MULTY_EOS_RPC",
		},
		cli.StringFlag{
			Name:   "p2p",
			Usage:  "node p2p address",
			EnvVar: "MULTY_EOS_P2P",
		},
		cli.StringFlag{
			Name:   "account",
			Usage:  "eos account for user registration",
			EnvVar: "MULTY_EOS_ACCOUNT",
		},
		cli.StringFlag{
			Name:   "key",
			Usage:  "active key for specified user for user registration",
			EnvVar: "MULTY_EOS_KEY",
		},
	}
	app.Action = run
	app.Run(os.Args)
}
