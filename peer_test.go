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
	"google.golang.org/grpc/connectivity"
	"testing"
)

func TestPeer_GetConnection(t *testing.T) {
	node := NewNode("localhost:9488")
	node.StartServer()
	defer node.StopServer()

	peer := Peer{Addr: node.Addr}
	conn, err := peer.GetConnection()
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	if state := conn.GetState(); state != connectivity.Idle {
		t.Errorf("failed to get connection (Expect IDLE): %v", state)
	}
}

func TestPeer_Disconnect(t *testing.T) {
	node := NewNode("localhost:9488")
	node.StartServer()
	defer node.StopServer()

	peer := Peer{Addr: node.Addr}

	if err := peer.Disconnect(); err != nil {
		t.Errorf("failed to disconnect unconnected peer: %v", err)
	}

	conn, err := peer.GetConnection()
	if err != nil {
		t.Error(err)
	}
	if state := conn.GetState(); state != connectivity.Idle {
		t.Errorf("failed to get connection (expect IDLE): %v", state)
	}

	if err := peer.Disconnect(); err != nil {
		t.Errorf("failed to disconnect peer: %v", err)
	}
	if state := conn.GetState(); state != connectivity.Shutdown {
		t.Errorf("failed to close connection (expect SHUTDOWN): %v", state)
	}
}

func TestPeer_GetPeers(t *testing.T) {
	node := NewNode("localhost:9488")
	node.StartServer()
	defer node.StopServer()

	nodePeer := Peer{Addr: node.Addr}
	testPeer := Peer{Addr: "localhost:9588"}
	peers, err := nodePeer.GetPeers(testPeer.Addr)
	if err != nil {
		t.Error(err)
	}
	if len(peers) != 0 {
		t.Errorf("failed to get peers (expect 0): %v", len(peers))
	}

	node.RemovePeer(testPeer.Addr)
	node.AddPeers(tests...)
	peers, err = nodePeer.GetPeers(testPeer.Addr)
	if err != nil {
		t.Error(err)
	}
	if len(peers) != len(tests) {
		t.Errorf("failed to get peers (expect %v): %v", len(tests), len(peers))
	}
}
