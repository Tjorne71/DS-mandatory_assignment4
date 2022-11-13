package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	ping "github.com/NaddiNadja/peer-to-peer/grpc"
	"google.golang.org/grpc"
)

const port = 8000
const timeoutMin = 1000
const timeoutMax = 1500
var lockAt int32
var timestamp int32
var promise int32
var promiseLock bool
var commitLock bool

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + port
	timestamp = 0
	fmt.Printf("ownport: %d\n", ownPort)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:            ownPort,
		amountOfPings: make(map[int32]int32),
		clients:       make(map[int32]ping.PingClient),
		ctx:           ctx,
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

	

	for i := 0; i < 5; i++ {
		port := int32(port) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		fmt.Printf("succes on dial: %v\n", port)
		defer conn.Close()
		c := ping.NewPingClient(conn)
		p.clients[port] = c
	}
	for true {
		time.Sleep(time.Duration(rand.Intn((timeoutMax - timeoutMin) + timeoutMin))* time.Millisecond)
		gotAcces := p.RequestFileAcces()
		if(gotAcces) {
			p.WriteToFile()
			p.ReleaseLock()
			time.Sleep(20*time.Second)
		}
	}
}

type peer struct {
	ping.UnimplementedPingServer
	id            int32
	amountOfPings map[int32]int32
	clients       map[int32]ping.PingClient
	ctx           context.Context
}

func getConnections() { 
	
}

func (p *peer) RequestFileAcces() bool{ 
	for true {
		if p.getPromices() {
			break
		}
	}
	fmt.Printf("%v got the promises\n", p.id)
	for true {
		if p.getCommits() {
			break
		}
	}
	if(lockAt == p.id) {
		fmt.Printf("%v got the lock\n", p.id)
		return true
	} else {
		return false
	}
}

func (p *peer) Lock(ctx context.Context, req *ping.LockRelease) (*ping.LockResponse, error) {
	fmt.Printf("Got told to release lock from %v\n", req.Id)
	lockAt = 0
	promise = 0
	promiseLock = false
	commitLock = false
	rep := &ping.LockResponse{Id: p.id, Timestamp: req.Timestamp}
	return rep, nil
}

func (p *peer) ReleaseLock() {
	request := &ping.LockRelease{Timestamp: timestamp, Id: p.id}
	for id, client := range p.clients {
		fmt.Printf("Tell %v to release lock\n", id)
		response, err := client.Lock(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
			continue
		}
		if(response.Timestamp == timestamp) {
			lockAt = 0
			promise = 0
			promiseLock = false
			commitLock = false
		}
	}
}

func (p *peer) Promise(ctx context.Context, req *ping.PromiseRequest) (*ping.PromiseResponce, error) {
	fmt.Printf("Got Promise Request From %v\n", req.Id)
	var rep *ping.PromiseResponce
	if(req.Timestamp > timestamp && !promiseLock) {
		fmt.Printf("Granted Promise To %v\n", req.Id)
		promiseLock = true
		promise = req.Id
		rep = &ping.PromiseResponce{Granted: true, Timestamp: req.Timestamp}
		timestamp = req.Timestamp
	} else {
		fmt.Printf("Didnt Grant Promise To %v (promiselock: %v) (timestamp: %v)\n", req.Id, promiseLock, req.Timestamp)
		rep = &ping.PromiseResponce{Granted: false, Timestamp: timestamp}
	}
	return rep, nil
}

func (p *peer) getPromices() bool {
	if(promiseLock == true) {
		return false
	}
	timestamp++
	request := &ping.PromiseRequest{Timestamp: timestamp, Id: p.id}
	promiseCount := 0
	for id, client := range p.clients {
		fmt.Printf("Get Promise From %v\n", id)
		response, err := client.Promise(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
			continue
		}
		if(response.Granted) {
			promiseCount += 1
			fmt.Printf("Got Promise From %v\n", id)
			if(promiseCount >= (len(p.clients)) + 1 / 2) {
				break
			}
		} else {
			timestamp = response.Timestamp
			fmt.Printf("Didnt get Promise From %v, new timestamp is %v\n", id, response.Timestamp)
		}
	}
	if(promiseCount >= (len(p.clients)) + 1 / 2) {
		promise = p.id
		return true
	} else {
		return false
	}
}

func (p *peer) Commit(ctx context.Context, req *ping.CommitRequest) (*ping.CommitResponse, error) {
	fmt.Printf("Got Commit Request From %v\n", req.Id)
	var rep *ping.CommitResponse
	if(req.Timestamp == timestamp && req.Id == promise) {
		fmt.Printf("Granted commit To %v\n", req.Id)
		commitLock = true
		rep = &ping.CommitResponse{Timestamp: timestamp, Id: req.Id}
	} else {
		fmt.Printf("Didnt Grant Commit To %v\n", req.Id)
		rep = &ping.CommitResponse{Timestamp: timestamp, Id: lockAt}
	}
	return rep, nil
}

func (p *peer) getCommits() bool {
	if(promise != p.id) {
		return false
	}
	request := &ping.CommitRequest{Timestamp: timestamp, Id: p.id}
	commitCount := 0
	for id, client := range p.clients {
		fmt.Printf("Get Commit From %v\n", id)
		response, err := client.Commit(p.ctx, request)
		if err != nil {
			fmt.Println("something went wrong")
			continue
		}
		if(response.Id == p.id) {
			fmt.Printf("Got Commit From %v\n", id)
			commitCount+=1
			if(commitCount >= (len(p.clients)) + 1 / 2) {
				break
			}
		} else {
			if(response.Id == id) {
				break;
			}
			timestamp = response.Timestamp
			fmt.Printf("Didnt get Commit From %v\n", id)
		}
	}
	if(commitCount >= (len(p.clients)) + 1 / 2) {
		lockAt = p.id
		return true
	} else {
		return false
	}
}

func (p *peer) WriteToFile() { 
	fmt.Printf("%v is entering critical section and writing to file\n", p.id)
	f, err := os.OpenFile("data.txt",
	os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
        log.Fatal(err)
    }
	for i := 0; i < 10; i++ {
		var string = fmt.Sprintf("%v wrote to file: %v (timestamp: %v)\n", p.id, i, timestamp)
		_, err2 := f.WriteString(string)

		if err2 != nil {
			log.Fatal(err2)
    }
	}
}