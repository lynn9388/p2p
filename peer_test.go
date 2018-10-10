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
	"strconv"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/connectivity"
)

var tests = []string{
	"127.0.0.1:9388",
	"127.0.0.1:9389",
	"127.0.0.1:9390",
	"127.0.0.1:9391",
}

func TestPeerManager_AddPeers(t *testing.T) {
	pm := NewPeerManager(tests[0])
	waiter := sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		waiter.Add(1)
		go func(port int) {
			pm.AddPeers("localhost:" + strconv.Itoa(port))
			waiter.Done()
		}(i)
	}
	waiter.Wait()

	if len(pm.Peers) != 10 {
		t.Error(len(pm.Peers))
	}
}

func TestPeerManager_RemovePeer(t *testing.T) {
	pm := NewPeerManager(tests[0])
	waiter := sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		pm.AddPeers("localhost:" + strconv.Itoa(i))
	}

	for i := 0; i < 5; i++ {
		waiter.Add(1)
		go func(port int) {
			if err := pm.RemovePeer("localhost:" + strconv.Itoa(port)); err != nil {
				t.Error(err)
			}
			waiter.Done()
		}(i)
	}
	waiter.Wait()

	if len(pm.Peers) != 5 {
		t.Error(len(pm.Peers))
	}
}

func TestPeerManager_GetConnection(t *testing.T) {
	node := NewNode(tests[0])
	node.StartServer()
	defer node.StopServer()

	pm := NewPeerManager(tests[1])
	conn, err := pm.GetConnection(node.Addr)
	if err != nil {
		t.Errorf("failed to get connection to unknown peer: %v", node.Addr)
	}

	pm.AddPeers(node.Addr)
	conn, err = pm.GetConnection(node.Addr)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	if state := conn.GetState(); state != connectivity.Idle {
		t.Errorf("failed to get connection to peer: %v %v(Expect IDLE)", node.Addr, state)
	}
}

func TestPeerManager_Disconnect(t *testing.T) {
	node := NewNode(tests[0])
	node.StartServer()
	defer node.StopServer()

	pm := NewPeerManager(tests[1])
	if err := pm.Disconnect(node.Addr); err == nil {
		t.Errorf("disconnect to unknown peer without error: %v", node.Addr)
	}

	pm.AddPeers(node.Addr)
	_, err := pm.GetConnection(node.Addr)
	if err != nil {
		t.Error(err)
	}
	if state := pm.Peers[node.Addr].conn.GetState(); state != connectivity.Idle {
		t.Errorf("failed to get connection to peer: %v %v(expect IDLE)", node.Addr, state)
	}
	if err := pm.Disconnect(node.Addr); err != nil {
		t.Errorf("failed to disconnect peer: %v: %v", node.Addr, err)
	}
	if state := pm.Peers[node.Addr].conn.GetState(); state != connectivity.Shutdown {
		t.Errorf("failed to disconnect peer: %v %v(expect SHUTDOWN)", node.Addr, state)
	}
}

func TestPeerManager_DisconnectAll(t *testing.T) {
	for _, addr := range tests {
		node := NewNode(addr)
		node.StartServer()
		defer node.StopServer()
	}

	pm := NewPeerManager("localhost:9488")
	if err := pm.Disconnect(tests[0]); err == nil {
		t.Errorf("disconnect to unknown peer without error: %v", tests[0])
	}

	pm.AddPeers(tests...)
	for _, addr := range tests {
		conn, err := pm.GetConnection(addr)
		if err != nil {
			t.Error(err)
		}

		if state := conn.GetState(); state != connectivity.Idle {
			t.Errorf("failed to get connection to peer: %v %v(expect IDLE)", addr, state)
		}
	}

	pm.DisconnectAll()
	for _, addr := range tests {
		if state := pm.Peers[addr].conn.GetState(); state != connectivity.Shutdown {
			t.Errorf("failed to disconnect peer: %v %v(expect SHUTDOWN)", addr, state)
		}
	}
}

func TestPeerManager_discoverPeers(t *testing.T) {
	server := NewNode("localhost:9488")
	server.StartServer()
	defer server.StopServer()

	pm := NewPeerManager("localhost:9588")

	pm.AddPeers(server.Addr)
	pm.discoverPeers(server.Addr)
	if len(pm.Peers) != 1 {
		t.Errorf("failed to get neighbor peers: %v(expect 0)", len(pm.Peers))
	}

	server.PeerManager.RemovePeer(pm.addr)
	server.PeerManager.AddPeers(tests...)
	pm.discoverPeers(server.Addr)
	if len(pm.Peers) != len(tests)+1 {
		t.Errorf("failed to get neighbor peers: %v(expect %v)", len(pm.Peers), len(tests)+1)
	}
}

func TestPeerManager_StartDiscoverPeers(t *testing.T) {
	for _, addr := range tests {
		node := NewNode(addr)
		node.StartServer()
		defer node.StopServer()
		node.PeerManager.StartDiscoverPeers(tests[0])
		defer node.PeerManager.StopDiscoverPeers()
	}

	node := NewNode("localhost:9488")
	node.StartServer()
	defer node.StopServer()
	node.PeerManager.StartDiscoverPeers(tests[0])
	defer node.PeerManager.StopDiscoverPeers()
	time.Sleep(5 * time.Second)

	if node.PeerManager.GetPeersNum() != len(tests) {
		t.Errorf("failed to join the network (expect %v): %v", len(tests), node.PeerManager.GetPeersNum())
	}
}

func TestPeerManager_StopDiscoverPeers(t *testing.T) {
	for _, addr := range tests {
		node := NewNode(addr)
		node.StartServer()
		defer node.StopServer()
		node.PeerManager.StartDiscoverPeers(tests[0])
		defer node.PeerManager.StopDiscoverPeers()
	}

	node := NewNode("localhost:9488")
	node.StartServer()
	defer node.StopServer()
	node.PeerManager.StartDiscoverPeers(tests[0])
	time.Sleep(5 * time.Second)

	node.PeerManager.StopDiscoverPeers()
	for _, addr := range node.PeerManager.GetPeers() {
		if state := node.PeerManager.GetPeerState(addr); state != connectivity.Shutdown {
			t.Errorf("failed to leave network: %v ", state)
		}
	}
}
