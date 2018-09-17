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
	"context"
	"fmt"
	"sync"

	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Peer is the the remote node that a local node can connect to.
type Peer struct {
	Addr string
	conn *grpc.ClientConn
}

// PeerManager manages the peers thant a local node known.
type PeerManager struct {
	self  string           // address of node
	Peers map[string]*Peer // known remote peers
	Mux   sync.RWMutex     // mutual exclusion lock for peers
}

// NewPeerManager returns a new peer manager with its own network address.
func NewPeerManager(self string) *PeerManager {
	return &PeerManager{
		self:  self,
		Peers: make(map[string]*Peer),
		Mux:   sync.RWMutex{},
	}
}

// AddPeers adds peers to the peer manager if a peer's network address is
// unknown before.
func (pm *PeerManager) AddPeers(addresses ...string) {
	pm.Mux.Lock()
	defer pm.Mux.Unlock()

	for _, addr := range addresses {
		if addr != pm.self {
			if _, ok := pm.Peers[addr]; !ok {
				pm.Peers[addr] = &Peer{Addr: addr}
				log.Debugf("%v adds peer: %v", pm.self, addr)
			}
		}
	}
}

// RemovePeer removes a peer from the peer manager. It disconnects the
// connection relative to the peer before removing.
func (pm *PeerManager) RemovePeer(addr string) error {
	pm.Mux.Lock()
	defer pm.Mux.Unlock()

	if _, ok := pm.Peers[addr]; ok {
		if err := pm.disconnect(addr); err != err {
			return err
		}

		delete(pm.Peers, addr)
		log.Debugf("%v removes peer: %v", pm.self, addr)
	}
	return nil
}

// getPeers returns all the peers in the peer manager.
func (pm *PeerManager) getPeers() []Peer {
	pm.Mux.RLock()
	defer pm.Mux.RUnlock()

	var ps []Peer
	for _, p := range pm.Peers {
		ps = append(ps, *p)
	}
	return ps
}

// getPeersNum returns the number of peers in the peer manager.
func (pm *PeerManager) getPeersNum() int {
	pm.Mux.RLock()
	defer pm.Mux.RUnlock()
	return len(pm.Peers)
}

// GetConnection returns a connection to a peer if the peer is known.
func (pm *PeerManager) GetConnection(addr string) (*grpc.ClientConn, error) {
	pm.Mux.Lock()
	defer pm.Mux.Unlock()

	p, ok := pm.Peers[addr]
	if !ok {
		return nil, fmt.Errorf("failed to get connection: unknown peer: %v", addr)
	}

	var state connectivity.State
	if p.conn != nil {
		state = p.conn.GetState()

		if state != connectivity.Idle && state != connectivity.Ready {
			if err := pm.Disconnect(addr); err != nil {
				return nil, err
			}
		}
	}

	if p.conn == nil || state != connectivity.Idle && state != connectivity.Ready {
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		p.conn = conn
		log.Debugf("%v connected to peer: %v", pm.self, addr)
	}
	return p.conn, nil
}

// disconnect closes the connection to the peer. This function is not thread-safe,
func (pm *PeerManager) disconnect(addr string) error {
	p, ok := pm.Peers[addr]
	if !ok {
		return fmt.Errorf("failed to disconnect: unknown peer: %v", addr)
	}

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return fmt.Errorf("failed to disconnect: %v", err)
		}
		log.Debugf("%v disconnected to peer: %v", pm.self, addr)
	}
	return nil
}

// Disconnect closes the connection to the peer.
func (pm *PeerManager) Disconnect(addr string) error {
	pm.Mux.Lock()
	defer pm.Mux.Unlock()

	return pm.disconnect(addr)
}

// GetNeighbors requests other neighbor peers from a peer.
func (pm *PeerManager) GetNeighbors(addr string) ([]string, error) {
	conn, err := pm.GetConnection(addr)
	if err != nil {
		return nil, err
	}

	client := NewNodeServiceClient(conn)
	peers, err := client.GetPeers(context.Background(), &wrappers.StringValue{Value: pm.self})
	if err != nil {
		return nil, fmt.Errorf("failed to get neighbors of peer: %v: %v", addr, err)
	}
	return peers.Peers, nil
}
