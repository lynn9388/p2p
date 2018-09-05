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

// Node is a independent entity in the P2P network.
type Node struct {
	self  Peer
	peers map[string]Peer
}

// NewNode initials a new node with specific host IP and port.
func NewNode(host string, port int) Node {
	return Node{
		self:  Peer{Host: host, Port: int32(port)},
		peers: make(map[string]Peer),
	}
}

// IsSelf checks if the node has same network address with a peer.
func (n *Node) IsSelf(p *Peer) bool {
	return n.self.Host == p.Host && n.self.Port == p.Port
}

// AddPeer adds a peer to the node's peer list.
func (n *Node) AddPeer(p *Peer) {
	if !n.IsSelf(p) {
		n.peers[p.GetAddr()] = *p
	}
}

// AddPeer removes a peer from the node's peer list.
func (n *Node) RemovePeer(p *Peer) {
	delete(n.peers, p.GetAddr())
}

// HasPeer checks if a peer is in the node's peer list.
func (n *Node) HasPeer(p *Peer) bool {
	_, ok := n.peers[p.GetAddr()]
	return ok
}
