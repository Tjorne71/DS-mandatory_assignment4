package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	ping "github.com/NaddiNadja/peer-to-peer/grpc"
	"google.golang.org/grpc"
)

const initialPort = 9000


func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + initialPort

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &Peer{
		id:            ownPort,
		amountOfPings: make(map[int32]int32),
		clients:       make(map[int32]ping.PingClient),
		ctx:           ctx,
		state:         RELEASED,
		lamportTime:   0,
		replyQueue:    make([]int, 0),
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	ping.RegisterPingServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(initialPort) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s\n", err)
		}
		fmt.Printf("Succes on dial: %v\n", port)
		defer conn.Close()
		c := ping.NewPingClient(conn)
		p.clients[port] = c
	}

	for true {
		if DecideToAccessFile() {
			fmt.Printf("I want to access the file: %v (lamport: %v)\n", ownPort, p.lamportTime)
			p.RequestAccess()
			if (p.state == HELD) {
				p.AccessToFile()
			}	
		} else {
			fmt.Printf("I dont want to access the file: %v (lamport: %v)\n", ownPort, p.lamportTime)
			time.Sleep(time.Second * 10)
		}
	}
}

type State int

const (
	WANTED State = iota
	HELD
	RELEASED
)

type Peer struct {
	ping.UnimplementedPingServer
	id            int32
	amountOfPings map[int32]int32
	clients       map[int32]ping.PingClient
	ctx           context.Context
	state         State
	lamportTime   int
	replyQueue    []int
}

// func (p *peer) Ping(ctx context.Context, req *ping.Request) (*ping.Reply, error) {
// 	id := req.Id
// 	p.amountOfPings[id] += 1

// 	rep := &ping.Reply{Amount: p.amountOfPings[id]}
// 	return rep, nil
// }

// func (p *peer) SendPingToAll() {
// 	request := &ping.Request{Id: p.id}
// 	for id, client := range p.clients {
// 		reply, err := client.Ping(p.ctx, request)
// 		if err != nil {
// 			fmt.Println("something went wrong")
// 		}
// 		fmt.Printf("Got reply from id %v: %v\n", id, reply.Amount)
// 	}
// }

func DecideToAccessFile() bool {
	randomNumber := GenerateRandomNumber(1,3)
	if randomNumber == 1 {
		return true
	}
	return false
}

func (p *Peer) RequestAccess() {
	p.state = WANTED
	request := &ping.Request{ProcessId: p.id}
	replies := 1
	for id, client := range p.clients {
		fmt.Printf("Request acces from %v (lamport: %v)\n", id, p.lamportTime)
		p.lamportTime++
		reply, err := client.RequestAccessToFile(p.ctx, request)
		p.OnLamportRecieved(int(reply.LamportTime))
		if err != nil {
			fmt.Println("something went wrong")
		}
		fmt.Printf("Got Reply from %v (lamport: %v) \n", id, p.lamportTime)
		replies++

		if(replies == len(p.clients)) {
			fmt.Printf("Holding access to file %v (lamport: %v)\n", p.id, p.lamportTime)
			p.state = HELD
			break
		}
	}
}

func (p *Peer) RequestAccessToFile(ctx context.Context, req *ping.Request) (*ping.Reply, error) {
	if(p.state == HELD || (p.state == WANTED && (p.lamportTime < int(req.LamportTime) || p.id > req.ProcessId))) {
		p.replyQueue = append(p.replyQueue, int(req.ProcessId))
		for p.state == HELD {}
	}
	p.OnLamportRecieved(int(req.LamportTime))
	rep := &ping.Reply{
		ProcessId: p.id,
		LamportTime: int32(p.lamportTime),
	}
	return rep, nil		
}

func (p *Peer) OnLamportRecieved(newT int) {
	p.lamportTime = int(math.Max(float64(p.lamportTime), float64(newT)) + 1)
}

func (p *Peer) AccessToFile() {
	fmt.Printf("%v: Has acces to file (CRITICAL SECTION) (lamport: %v)\n", p.id, p.lamportTime)
	time.Sleep(time.Second*time.Duration(GenerateRandomNumber(5, 10)))
	fmt.Printf("%v: Released acces to file (CRITICAL SECTION END) (lamport: %v)\n", p.id, p.lamportTime)
	p.state = RELEASED
}

func GenerateRandomNumber(min int, max int) int { 
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}



