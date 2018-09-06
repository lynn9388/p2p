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
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/wrappers"

	"github.com/golang/protobuf/ptypes/empty"
)

func TestNode_IsSelf(t *testing.T) {
	for addr, peer := range tests {
		node := NewNode(peer.Host, int(peer.Port))
		for a, p := range tests {
			if a == addr && !node.IsSelf(&p) {
				t.Errorf("IsSelft of %v = false", addr)
			}
		}
	}
}

func TestNode_HasPeer(t *testing.T) {
	node := NewNode("localhost", 9388)
	for addr, peer := range tests {
		if node.HasPeer(&peer) {
			t.Errorf("%v HasPeer %v = true", node, addr)
		}

		node.AddPeers(peer)
		if !node.HasPeer(&peer) {
			t.Errorf("%v HasPeer %v = false", node, addr)
		}

		node.RemovePeer(&peer)
		if node.HasPeer(&peer) {
			t.Errorf("%v HasPeer %v = true", node, addr)
		}
	}
}

func TestNode_getPeers(t *testing.T) {
	node := NewNode("localhost", 9388)
	for _, p := range tests {
		node.AddPeers(p)
	}

	peers := node.getPeers()
	for _, p := range tests {
		var exists bool
		for _, peer := range peers {
			if peer.GetAddr() == p.GetAddr() {
				exists = true
				break
			}
		}
		if !exists {
			t.Errorf("not exist %v", p.GetAddr())
		}
	}
}

type hello struct{}

var helloWorld = "Hello, world!"

func (h *hello) Hello(context.Context, *empty.Empty) (*wrappers.StringValue, error) {
	return &wrappers.StringValue{Value: helloWorld}, nil
}

func TestNode_RegisterService(t *testing.T) {
	node1 := NewNode("localhost", 9388)
	node2 := NewNode("localhost", 9389)
	node1.RegisterService(&_TestService_serviceDesc, &hello{})
	node1.StartServer()
	defer node1.StopServer()

	conn, err := node2.GetConnection(&node1.self)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	client := NewTestServiceClient(conn)
	hello, err := client.Hello(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	if hello.Value != helloWorld {
		t.Fail()
	}
}

func TestNode_GetConnection(t *testing.T) {
	node1 := NewNode("localhost", 9388)
	node2 := NewNode("localhost", 9389)
	node1.StartServer()
	defer node1.StopServer()

	conn, err := node2.GetConnection(&node1.self)
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	if state := conn.GetState().String(); state != "IDLE" {
		t.Error(state)
	}
}

func TestNode_JoinNetwork(t *testing.T) {
	node1 := NewNode("localhost", 9388)
	node2 := NewNode("localhost", 9389)
	node1.StartServer()
	defer node1.StopServer()

	var peers []Peer
	for _, p := range tests {
		peers = append(peers, p)
	}
	node1.AddPeers(peers...)

	node2.JoinNetwork(node1.self)
	time.Sleep(1 * time.Second)
	node2.LeaveNetwork()
	if node2.getPeersNum() != len(tests)+1 {
		t.Fail()
	}
}

func TestNode_Ping(t *testing.T) {
	node1 := NewNode("localhost", 9388)
	node2 := NewNode("localhost", 9389)
	node1.StartServer()
	defer node1.StopServer()

	conn, err := node2.GetConnection(&node1.self)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := NewNodeServiceClient(conn)
	pong, err := client.Ping(context.Background(), &PingPong{Message: PingPong_PING})
	if err != nil {
		t.Fatal(err)
	}

	if pong.Message != PingPong_PONG {
		t.Fatalf("invalid pong message: %v", pong.Message)
	}
}

func TestNode_GetPeers(t *testing.T) {
	node1 := NewNode("localhost", 9390)
	node2 := NewNode("localhost", 9389)
	node1.StartServer()
	node1.AddPeers(node2.self)
	defer node1.StopServer()

	conn, err := node2.GetConnection(&node1.self)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := NewNodeServiceClient(conn)
	peers, err := client.GetPeers(context.Background(), &empty.Empty{})
	if err != nil {
		t.Fatal(err)
	}

	if len(peers.GetPeers()) != 1 {
		t.Fail()
	}
}
