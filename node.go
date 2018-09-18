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
	"flag"
	"net"
	"sync"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Node is a independent entity in the P2P network.
type Node struct {
	Addr        string       // network address
	Server      *grpc.Server // gRPC server
	PeerManager              // peer manager

	leave  chan struct{}  // leave network signal
	waiter sync.WaitGroup // wait background goroutines
}

var (
	log *zap.SugaredLogger // default logger

	maxSleepTime time.Duration // sleep time between discover neighbor peers
	maxPeerNum   int           // max neighbor peers' number
)

func init() {
	logger, _ := zap.NewDevelopment()
	log = logger.Sugar()

	if flag.Lookup("test.v") != nil { // go test
		maxSleepTime = 1 * time.Second
		maxPeerNum = 5
	} else {
		maxSleepTime = 5 * time.Second
		maxPeerNum = 20
	}
}

// NewNode initials a new node with specific network address.
func NewNode(addr string) Node {
	return Node{
		Addr:        addr,
		Server:      grpc.NewServer(),
		PeerManager: *NewPeerManager(addr),

		leave:  make(chan struct{}),
		waiter: sync.WaitGroup{},
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

// JoinNetwork discovers new peers via bootstraps until there are enough
// peers in peer list.
func (n *Node) JoinNetwork(bootstraps ...string) {
	n.AddPeers(bootstraps...)

	n.waiter.Add(1)
	go func() {
		for {
			if n.getPeersNum() < maxPeerNum {
				for _, p := range n.getPeers() {
					peers, err := n.RequestNeighbors(p.Addr)
					if err != nil {
						log.Error(err)
						continue
					}
					n.AddPeers(peers...)
					if n.getPeersNum() > maxPeerNum {
						break
					}
				}
			}

			select {
			case <-n.leave:
				n.waiter.Done()
				n.leave <- struct{}{}
				return
			case <-time.After(maxSleepTime):
				continue
			}
		}
	}()
}

// LeaveNetwork stops discovering new peers and disconnect all connections.
func (n *Node) LeaveNetwork() {
	// request to stop discovering new peers
	n.leave <- struct{}{}

	// wait for discovering new peers stopped
	<-n.leave

	n.Mux.Lock()
	for _, p := range n.Peers {
		if err := n.disconnect(p.Addr); err != nil {
			log.Error(err)
		}
	}
	n.Mux.Unlock()
}

// Wait keeps node running in background.
func (n *Node) Wait() {
	n.waiter.Wait()
}

// GetNeighbors returns the peers known by a node.
func (n *Node) GetNeighbors(ctx context.Context, addr *wrappers.StringValue) (*Peers, error) {
	var peers []string
	for _, p := range n.getPeers() {
		peers = append(peers, p.Addr)
	}

	n.AddPeers(addr.Value)
	return &Peers{Peers: peers}, nil
}
