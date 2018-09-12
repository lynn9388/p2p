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

package p2p

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const maxSleepTime = 5 * time.Second
const maxConnNum = 20

// Node is a independent entity in the P2P network.
type Node struct {
	self         Peer
	server       *grpc.Server
	leaveNetwork chan struct{}
	peers        map[string]Peer
	connections  map[string]*grpc.ClientConn
	mux          sync.RWMutex
	waiter       sync.WaitGroup
}

// NewNode initials a new node with specific host IP and port.
func NewNode(host string, port int) Node {
	return Node{
		self:         Peer{Host: host, Port: int32(port)},
		server:       grpc.NewServer(),
		leaveNetwork: make(chan struct{}),
		peers:        make(map[string]Peer),
		connections:  make(map[string]*grpc.ClientConn),
		mux:          sync.RWMutex{},
		waiter:       sync.WaitGroup{},
	}
}

// IsSelf checks if the node has same network address with a peer.
func (n *Node) IsSelf(p *Peer) bool {
	return n.self.Host == p.Host && n.self.Port == p.Port
}

// AddPeers adds peers to the node's peer list.
func (n *Node) AddPeers(ps ...Peer) {
	n.mux.Lock()
	defer n.mux.Unlock()
	for _, p := range ps {
		if !n.IsSelf(&p) {
			if _, ok := n.peers[p.GetAddr()]; !ok {
				n.peers[p.GetAddr()] = p
				log.Printf("%v add peer: %v", n.self.GetAddr(), p.GetAddr())
			}
		}
	}
}

// RemovePeer removes a peer from the node's peer list.
func (n *Node) RemovePeer(p *Peer) {
	n.disconnectPeer(p)
	n.mux.Lock()
	delete(n.peers, p.GetAddr())
	n.mux.Unlock()
}

// HasPeer checks if a peer is in the node's peer list.
func (n *Node) HasPeer(p *Peer) bool {
	n.mux.RLock()
	_, ok := n.peers[p.GetAddr()]
	n.mux.RUnlock()
	return ok
}

// getPeers returns all the peers in the node's peer list.
func (n *Node) getPeers() []Peer {
	var ps []Peer

	n.mux.RLock()
	for _, p := range n.peers {
		ps = append(ps, p)
	}
	n.mux.RUnlock()

	return ps
}

// getPeersNum returns the number of peers in the node's peer list.
func (n *Node) getPeersNum() int {
	n.mux.RLock()
	defer n.mux.RUnlock()
	return len(n.peers)
}

// StartServer starts server to serve node request.
func (n *Node) StartServer() {
	lis, err := net.Listen("tcp", n.self.GetAddr())
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// register internal service
	RegisterNodeServiceServer(n.server, n)

	log.Printf("server is listening at: %v", n.self.GetAddr())
	go n.server.Serve(lis)
}

// StopServer closes all connections and stop the server.
func (n *Node) StopServer() {
	n.mux.Lock()
	defer n.mux.Unlock()
	for _, conn := range n.connections {
		conn.Close()
	}

	if n.server != nil {
		n.server.Stop()
		log.Print("server stopped")
	}
}

// connectPeer connects a peer if the node hasn't connected to it, or
// reconnects the peer otherwise.
func (n *Node) connectPeer(p *Peer) (*grpc.ClientConn, error) {
	n.disconnectPeer(p)
	conn, err := grpc.Dial(p.GetAddr(), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	n.mux.Lock()
	n.connections[p.GetAddr()] = conn
	n.mux.Unlock()
	return conn, nil
}

// disconnectPeer disconnects the connection if the node has connected to the
// peer.
func (n *Node) disconnectPeer(p *Peer) {
	n.mux.Lock()
	defer n.mux.Unlock()
	conn, ok := n.connections[p.GetAddr()]
	if ok {
		conn.Close()
	}
}

// GetConnection returns a connection between the node and the peer.
func (n *Node) GetConnection(p *Peer) (*grpc.ClientConn, error) {
	n.mux.RLock()
	conn, ok := n.connections[p.GetAddr()]
	n.mux.RUnlock()
	if !ok {
		var err error
		conn, err = n.connectPeer(p)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil
}

// discoverPeers requests other neighbor peers from a connected peer.
func (n *Node) discoverPeers(p *Peer) ([]Peer, error) {
	var ps []Peer

	conn, err := n.GetConnection(p)
	if err != nil {
		return nil, err
	}

	client := NewNodeServiceClient(conn)
	peers, err := client.GetPeers(context.Background(), &n.self)
	if err != nil {
		return nil, err
	}

	for _, p := range peers.Peers {
		ps = append(ps, *p)
	}

	return ps, nil
}

// JoinNetwork discovers new peers via bootstraps until have enough peers in
// peer list.
func (n *Node) JoinNetwork(bootstraps ...Peer) {
	n.AddPeers(bootstraps...)

	n.waiter.Add(1)
	go func() {
		for {
			if n.getPeersNum() < maxConnNum {
				for _, p := range n.getPeers() {
					peers, err := n.discoverPeers(&p)
					if err != nil {
						log.Print(err)
						continue
					}
					n.AddPeers(peers...)
					if n.getPeersNum() > maxConnNum {
						break
					}
				}
			}

			select {
			case <-n.leaveNetwork:
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
	n.leaveNetwork <- struct{}{}

	n.mux.Lock()
	defer n.mux.Unlock()
	for _, conn := range n.connections {
		conn.Close()
	}
}

// Wait keeps node running in background.
func (n *Node) Wait() {
	n.waiter.Wait()
}

// Ping returns pong message when received ping message.
func (n *Node) Ping(ctx context.Context, ping *PingPong) (*PingPong, error) {
	if ping.Message != PingPong_PING {
		return nil, errors.New("invalid ping message: " + ping.Message.String())
	}
	return &PingPong{Message: PingPong_PONG}, nil
}

// GetPeers return a list of peer to client.
func (n *Node) GetPeers(ctx context.Context, p *Peer) (*Peers, error) {
	var peers []*Peer

	ps := n.getPeers()
	for i := range ps {
		peers = append(peers, &ps[i])
	}

	n.AddPeers(*p)
	return &Peers{Peers: peers}, nil
}
