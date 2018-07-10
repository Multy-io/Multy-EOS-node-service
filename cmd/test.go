package main

import (
	"github.com/Appscrunch/Multy-Back-EOS/proto"
	"google.golang.org/grpc"
	"log"
	"context"
)

func main() {
	log.Println("start")
	conn, err := grpc.Dial("144.76.203.79:32811", grpc.WithInsecure())
	log.Println("dial")
	if err != nil {
		log.Fatal(err)
	}
	client := proto.NewNodeCommunicationsClient(conn)
	ctx := context.Background()
	//resp, err := client.EventGetChainInfo(ctx, &proto.Empty{})
	resp, err := client.EventGetAccount(ctx, &proto.Account{"vovapi"})
	//resp, err := client.EventGetBalance(ctx, &proto.BalanceReq{Name:"vovapi"})
	log.Printf("resp %v %s", resp, err)

}
