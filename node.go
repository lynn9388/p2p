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

	"github.com/golang/protobuf/ptypes/empty"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// Node is a independent entity in the P2P network.
type Node struct {
	self        Peer
	server      *grpc.Server
	peers       map[string]Peer
	connections map[string]*grpc.ClientConn
	mux         sync.RWMutex
}

// NewNode initials a new node with specific host IP and port.
func NewNode(host string, port int) Node {
	return Node{
		self:        Peer{Host: host, Port: int32(port)},
		server:      grpc.NewServer(),
		peers:       make(map[string]Peer),
		connections: make(map[string]*grpc.ClientConn),
	}
}

// IsSelf checks if the node has same network address with a peer.
func (n *Node) IsSelf(p *Peer) bool {
	return n.self.Host == p.Host && n.self.Port == p.Port
}

// AddPeer adds a peer to the node's peer list.
func (n *Node) AddPeer(p *Peer) {
	if !n.IsSelf(p) {
		n.mux.Lock()
		n.peers[p.GetAddr()] = *p
		n.mux.Unlock()
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

// RegisterService register external service. this must be called before
// starting server.
func (n *Node) RegisterService(desc *grpc.ServiceDesc, srv interface{}) {
	n.server.RegisterService(desc, srv)
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

// Ping returns pong message when received ping message.
func (n *Node) Ping(ctx context.Context, ping *PingPong) (*PingPong, error) {
	if ping.Message != PingPong_PING {
		return nil, errors.New("invalid ping message: " + ping.Message.String())
	}
	return &PingPong{Message: PingPong_PONG}, nil
}

// GetPeers return a list of peer to client.
func (n *Node) GetPeers(context.Context, *empty.Empty) (*Peers, error) {
	peers := make([]*Peer, 0)

	for _, p := range n.peers {
		peers = append(peers, &p)
	}

	return &Peers{Peers: peers}, nil
}
