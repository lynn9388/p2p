/*
 * Copyright © 2018 Lynn <lynn9388@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package p2p implements a node in P2P network.
package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/golang/protobuf/ptypes/wrappers"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Node is a independent entity in the P2P network.
type Node struct {
	Addr           string          // network address
	Server         *grpc.Server    // gRPC server
	PeerManager    *PeerManager    // peer manager
	MessageManager *MessageManager // message manager

	stopDiscover    chan struct{}  // stop discover neighbor peers signal
	discoverStopped chan struct{}  // discover neighbor peers stopped signal
	waiter          sync.WaitGroup // wait background goroutines

	messages sync.Map // hash and time of recent received messages
}

var (
	log *zap.SugaredLogger // default logger

	maxRequestTime time.Duration // timeout for request rpc
	maxSleepTime   time.Duration // sleep time between discover neighbor peers
	maxPeerNum     int           // max neighbor peers' number
)

func init() {
	logger, _ := zap.NewDevelopment()
	log = logger.Sugar()
	maxRequestTime = 5 * time.Second

	if flag.Lookup("test.v") != nil { // go test
		maxSleepTime = 2 * time.Second
		maxPeerNum = 5
	} else {
		maxSleepTime = 5 * time.Second
		maxPeerNum = 20
	}
}

// NewNode initials a new node with specific network address.
func NewNode(addr string) *Node {
	return &Node{
		Addr:           addr,
		Server:         grpc.NewServer(),
		PeerManager:    NewPeerManager(addr),
		MessageManager: NewMessageManager(),

		stopDiscover:    make(chan struct{}),
		discoverStopped: make(chan struct{}),
		waiter:          sync.WaitGroup{},

		messages: sync.Map{},
	}
}

// StartServer starts server to provide services. This must be called after
// registering any other external service.
func (n *Node) StartServer() {
	conn, _ := net.DialTimeout("tcp", n.Addr, 5*time.Second)
	if conn != nil {
		conn.Close()
	}

	lis, err := net.Listen("tcp", n.Addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// register internal service
	RegisterNodeServiceServer(n.Server, n)
	RegisterMessageServiceServer(n.Server, n.MessageManager)

	log.Infof("server is listening at: %v", n.Addr)
	go n.Server.Serve(lis)
}

// StopServer stops the server.
func (n *Node) StopServer() {
	if n.Server != nil {
		n.Server.Stop()
		log.Infof("server stopped: %v", n.Addr)
	}
}

// StartDiscoverPeers starts discovering new peers via bootstraps.
func (n *Node) StartDiscoverPeers(bootstraps ...string) {
	n.PeerManager.AddPeers(bootstraps...)

	n.waiter.Add(1)
	go func() {
		for {
			if n.PeerManager.GetPeersNum() < maxPeerNum {
				for _, addr := range n.PeerManager.GetPeers() {
					peers, err := n.RequestNeighbors(addr)
					if err != nil {
						log.Error(err)
						continue
					}
					n.PeerManager.AddPeers(peers...)
					if n.PeerManager.GetPeersNum() >= maxPeerNum {
						break
					}
				}
			}

			select {
			case <-n.stopDiscover:
				n.waiter.Done()
				n.discoverStopped <- struct{}{}
				return
			case <-time.After(maxSleepTime):
				continue
			}
		}
	}()
}

// StopDiscoverPeers stops discovering new peers and disconnect all connections.
func (n *Node) StopDiscoverPeers() {
	// request to stop discovering new peers
	n.stopDiscover <- struct{}{}

	// wait for discovering new peers stopped
	<-n.discoverStopped

	n.PeerManager.DisconnectAll()
}

// Wait keeps node running in background.
func (n *Node) Wait() {
	n.waiter.Wait()
}

// GetNeighbors returns the peers known by a node.
func (n *Node) GetNeighbors(ctx context.Context, addr *wrappers.StringValue) (*Peers, error) {
	addresses := n.PeerManager.GetPeers()
	n.PeerManager.AddPeers(addr.Value)
	return &Peers{Peers: addresses}, nil
}

// RequestNeighbors requests other neighbor peers from a peer.
func (n *Node) RequestNeighbors(addr string) ([]string, error) {
	conn, err := n.PeerManager.GetConnection(addr)
	if err != nil {
		return nil, err
	}

	client := NewNodeServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), maxRequestTime)
	defer cancel()
	peers, err := client.GetNeighbors(ctx, &wrappers.StringValue{Value: n.Addr})
	if err != nil {
		return nil, fmt.Errorf("failed to get neighbors of peer: %v: %v", addr, err)
	}
	return peers.Peers, nil
}

// Broadcast receives message and broadcasts it to neighbor peers. The node
// will not broadcast messages with same content, so the messages should append
// extra info to identify messages.
func (n *Node) Broadcast(ctx context.Context, msg *any.Any) (*empty.Empty, error) {
	key := hash(msg.Value)
	if _, ok := n.messages.LoadOrStore(key, time.Now()); ok {
		log.Debugf("%v received duplicate message: %v", n.Addr, key)
		return &empty.Empty{}, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return &empty.Empty{}, errors.New("failed to get metadata from context")
	}
	addresses := md.Get("address")
	if len(addresses) != 1 {
		return &empty.Empty{}, errors.New("failed to get address of message sender from context")
	}

	log.Debugf("%v received message from peer: %v", n.Addr, addresses[0])

	for _, addr := range n.PeerManager.GetPeers() {
		if addr != addresses[0] {
			if err := n.RequestBroadcast(addr, msg); err != nil {
				log.Error(err)
			}
		}
	}
	return &empty.Empty{}, nil
}

// RequestBroadcast requests to broadcast a message to entire network.
func (n *Node) RequestBroadcast(addr string, msg *any.Any) error {
	conn, err := n.PeerManager.GetConnection(addr)
	if err != nil {
		return err
	}

	n.messages.Store(hash(msg.Value), time.Now())

	client := NewNodeServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), maxRequestTime)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("address", n.Addr))
	_, err = client.Broadcast(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to broadcast message to peer: %v: %v", addr, err)
	}

	log.Debugf("%v sends message to peer: %v", n.Addr, addr)
	return nil
}

// hash returns the hash value of data.
func hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
