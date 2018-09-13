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
	"flag"
	"github.com/golang/protobuf/ptypes/wrappers"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	maxSleepTime time.Duration
	maxPeerNum   int
)

// Node is a independent entity in the P2P network.
type Node struct {
	Addr   string       // local network address
	Server *grpc.Server // gRPC server

	Peers map[string]*Peer // known remote nodes
	Mux   sync.RWMutex     // mutual exclusion lock for peers

	leave  chan struct{}  // leave network signal
	waiter sync.WaitGroup // wait background goroutines
}

func init() {
	if flag.Lookup("test.v") == nil {
		maxSleepTime = 5 * time.Second
		maxPeerNum = 20
	} else {
		maxSleepTime = 1 * time.Second
		maxPeerNum = 5
	}
}

// NewNode initials a new node with specific host IP and port.
func NewNode(host string, port int) Node {
	return Node{
		Addr:   net.JoinHostPort(host, strconv.Itoa(port)),
		Server: grpc.NewServer(),

		Peers: make(map[string]*Peer),
		Mux:   sync.RWMutex{},

		leave:  make(chan struct{}),
		waiter: sync.WaitGroup{},
	}
}

// StartServer starts server to provide services. This must be called after
// registering any other external service.
func (n *Node) StartServer() {
	lis, err := net.Listen("tcp", n.Addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// register internal service
	RegisterNodeServiceServer(n.Server, n)

	log.Printf("server is listening at: %v", n.Addr)
	go n.Server.Serve(lis)
}

// StopServer closes all connections and stop the server.
func (n *Node) StopServer() {
	n.Mux.Lock()
	for _, p := range n.Peers {
		p.Disconnect()
	}
	n.Mux.Unlock()

	if n.Server != nil {
		n.Server.Stop()
		log.Print("server stopped")
	}
}

// AddPeers adds peers to the node's peer list if a peer's network address
// is unknown before.
func (n *Node) AddPeers(addresses ...string) {
	n.Mux.Lock()
	defer n.Mux.Unlock()

	for _, addr := range addresses {
		if n.Addr != addr {
			if _, ok := n.Peers[addr]; !ok {
				n.Peers[addr] = &Peer{Addr: addr}
				log.Printf("%v adds peer: %v", n.Addr, addr)
			}
		}
	}
}

// RemovePeer removes a peer from the node's peer list. It disconnects the
// connection relative to the peer before removing.
func (n *Node) RemovePeer(addr string) error {
	n.Mux.Lock()
	defer n.Mux.Unlock()

	if p, ok := n.Peers[addr]; ok {
		if err := p.Disconnect(); err != nil {
			return err
		}

		delete(n.Peers, p.Addr)
		log.Printf("%v removes peer: %v", n.Addr, p.Addr)
	}

	return nil
}

// getPeers returns all the peers in the node's peer list.
func (n *Node) getPeers() []Peer {
	var ps []Peer

	n.Mux.RLock()
	for _, p := range n.Peers {
		ps = append(ps, *p)
	}
	n.Mux.RUnlock()

	return ps
}

// getPeersNum returns the number of peers in the node's peer list.
func (n *Node) getPeersNum() int {
	n.Mux.RLock()
	defer n.Mux.RUnlock()
	return len(n.Peers)
}

// JoinNetwork discovers new peers via bootstraps until have enough peers
// in peer list.
func (n *Node) JoinNetwork(bootstraps ...string) {
	n.AddPeers(bootstraps...)

	n.waiter.Add(1)
	go func() {
		for {
			if n.getPeersNum() < maxPeerNum {
				for _, p := range n.getPeers() {
					peers, err := p.GetPeers(n.Addr)
					if err != nil {
						log.Print(err)
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
				return
			case <-time.After(maxSleepTime):
				continue
			}
		}
	}()
}

// LeaveNetwork stop discovering new peers and disconnect all connections.
func (n *Node) LeaveNetwork() {
	n.leave <- struct{}{}

	n.Mux.Lock()
	defer n.Mux.Unlock()
	for _, p := range n.Peers {
		p.Disconnect()
	}
}

// Wait keeps node running in background.
func (n *Node) Wait() {
	n.waiter.Wait()
}

// GetPeers return a list of known peer to client.
func (n *Node) GetPeers(ctx context.Context, addr *wrappers.StringValue) (*Peers, error) {
	var peers []string
	for _, p := range n.getPeers() {
		peers = append(peers, p.Addr)
	}

	n.AddPeers(addr.Value)
	return &Peers{Peers: peers}, nil
}
