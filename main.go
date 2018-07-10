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

	"github.com/Appscrunch/Multy-Back-EOS/eos"
	pb "github.com/Appscrunch/Multy-Back-EOS/proto"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
)

var (
	commit    string
	branch    string
	buildtime string
)

const (
	VERSION = "v0.2"
)

func run(c *cli.Context) error {
	server, err := eos.NewServer(
		c.String("node"),
		c.String("account"),
		c.String("key"),
	)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("cannot init server: %s", err), 2)
	}
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
	return cli.NewExitError(s.Serve(lis), 3)
}

func main() {
	app := cli.NewApp()
	app.Name = "multy-eos"
	app.Usage = `eos node gRPC API for Multy backend`
	app.Version = fmt.Sprintf("%s (commit: %s, branch: %s, buildtime: %s)", VERSION, commit, branch, buildtime)
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
			Name:   "node",
			Usage:  "node api address",
			EnvVar: "MULTY_EOS_NODE",
		},
		cli.StringFlag{
			Name:   "account",
			Usage:  "eosit account for user registration",
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