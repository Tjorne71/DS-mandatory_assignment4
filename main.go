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
		replyQueue:    make([]int32, 0),
		replies:       1,
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

	time.Sleep(time.Second * 2)

	for true {
		if DecideToAccessFile() {
			fmt.Printf("I want to access the file: %v (lamport: %v)\n", ownPort, p.lamportTime)
			p.RequestAcces()
			for p.state == WANTED {

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
	replyQueue    []int32
	replies       int
}

func DecideToAccessFile() bool {
	// randomNumber := GenerateRandomNumber(1,3)
	if 1 == 1 {
		return true
	}
	return false
}

func (p *Peer) RequestAcces() { 
	p.state = WANTED
	p.LamportTick()
	request := &ping.RequestMsg{ProcessId: p.id, LamportTime: int32(p.lamportTime)}
	for id, client := range p.clients {
		fmt.Printf("Sending Request Message To %v\n", id)
		_, err := client.Request(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
		}
	}
}

func (p *Peer) Request(ctx context.Context, req *ping.RequestMsg) (*ping.RequestRecievedMsg, error) {
	fmt.Printf("Recieved Request Message from %v\n", req.ProcessId)
	if(p.state == HELD || (p.state == WANTED && p.CompareRequest(req))) {
		fmt.Printf("Waited with responing to request from %v\n", req.ProcessId)
		p.OnLamportRecieved(int(req.LamportTime))
		p.replyQueue = append(p.replyQueue, req.ProcessId)
	} else {
		fmt.Printf("Responed to request Message from %v\n", req.ProcessId)
		p.OnLamportRecieved(int(req.LamportTime))
		reply := &ping.ReplyMsg{ProcessId: p.id, LamportTime: int32(p.lamportTime)}
		p.clients[req.ProcessId].Reply(ctx, reply)
	}
	rep := &ping.RequestRecievedMsg{ProcessId: p.id, LamportTime: int32(p.lamportTime)}
	return rep, nil
}

func (p *Peer) Reply(ctx context.Context, req *ping.ReplyMsg) (*ping.ReplyRecievedMsg, error) {
	fmt.Printf("Got a ReplyMsg from %v\n", req.ProcessId)
	p.replies++
	if(p.replies == len(p.clients)) {
		fmt.Printf("Got a Reply from all\n")
		p.AccessToFile()
	}
	rep := &ping.ReplyRecievedMsg{ProcessId: p.id, LamportTime: int32(p.lamportTime)}
	return rep, nil
}


func (p *Peer) CompareRequest(req *ping.RequestMsg) bool {
	if(p.lamportTime < int(req.LamportTime)) {
		return true
	} else if(p.lamportTime == int(req.LamportTime) && p.id > req.ProcessId) {
		return true
	}
	return false 
}

func (p *Peer) OnLamportRecieved(newT int) {
	p.lamportTime = int(math.Max(float64(p.lamportTime), float64(newT)) + 1)
}

func (p *Peer) LamportTick() {
	p.lamportTime += 1
}

func (p *Peer) AccessToFile() {
	fmt.Printf("%v: Has acces to file (CRITICAL SECTION) (lamport: %v)\n", p.id, p.lamportTime)
	p.WriteToFile()
	time.Sleep(time.Second*time.Duration(GenerateRandomNumber(5, 10)))
	fmt.Printf("%v: Released acces to file (CRITICAL SECTION END) (lamport: %v)\n", p.id, p.lamportTime)
	p.state = RELEASED
	p.replies = 0
	p.ReplyToAllInQueue()
}

func (p *Peer) ReplyToAllInQueue() {
	fmt.Printf("Emptying queued requests %v\n", p.replyQueue)
	for len(p.replyQueue) != 0 {
		reply := &ping.ReplyMsg{ProcessId: p.id, LamportTime: int32(p.lamportTime)}
		replyTo := p.replyQueue[0]
		fmt.Printf("Replying to %v\n", replyTo)
		p.clients[replyTo].Reply(p.ctx, reply)
		p.replyQueue = p.replyQueue[1:]
	}
}

func GenerateRandomNumber(min int, max int) int { 
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}



func (p *Peer) WriteToFile() { 
	f, err := os.OpenFile("data.txt",
	os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
        log.Fatal(err)
    }
	for i := 0; i < 10; i++ {
		var string = fmt.Sprintf("%v wrote to file: %v (timestamp: %v)\n", p.id, i, p.lamportTime)
		_, err2 := f.WriteString(string)

		if err2 != nil {
			log.Fatal(err2)
    }
	}
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