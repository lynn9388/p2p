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
	"strconv"
	"sync"
	"testing"
	"time"
)

var tests = []string{
	"127.0.0.1:9388",
	"192.168.1.1:9388",
}

func TestNode_AddPeers(t *testing.T) {
	node := NewNode("localhost", 9388)
	waiter := sync.WaitGroup{}

	for i := 0; i < 10; i++ {
		waiter.Add(1)
		go func(port int) {
			node.AddPeers("localhost:" + strconv.Itoa(port))
			waiter.Done()
		}(i)
	}
	waiter.Wait()

	if len(node.Peers) != 10 {
		t.Error(len(node.Peers))
	}
}

func TestNode_RemovePeer(t *testing.T) {
	node := NewNode("localhost", 9388)

	for i := 0; i < 10; i++ {
		node.AddPeers("localhost:" + strconv.Itoa(i))
	}

	waiter := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		waiter.Add(1)
		go func(port int) {
			if err := node.RemovePeer("localhost:" + strconv.Itoa(port)); err != nil {
				t.Error(err)
			}
			waiter.Done()
		}(i)
	}
	waiter.Wait()
	if len(node.Peers) != 5 {
		t.Error(len(node.Peers))
	}
}

func TestNode_JoinNetwork(t *testing.T) {
	node1 := NewNode("localhost", 9388)
	node2 := NewNode("localhost", 9488)

	node1.StartServer()
	defer node1.StopServer()
	node1.AddPeers(tests...)

	node2.JoinNetwork(node1.Addr)
	defer node2.LeaveNetwork()
	time.Sleep(1 * time.Second)
	if node2.getPeersNum() != len(tests)+1 {
		t.Errorf("failed to join the network: %v", node2.getPeersNum())
	}
}

func TestNode_LeaveNetwork(t *testing.T) {
	node1 := NewNode("localhost", 9388)
	node2 := NewNode("localhost", 9488)

	node1.StartServer()
	defer node1.StopServer()
	node1.AddPeers(tests...)

	node2.JoinNetwork(node1.Addr)
	time.Sleep(1 * time.Second)

	for _, p := range node2.Peers {
		p.GetConnection()
	}
	node2.LeaveNetwork()
	for _, p := range node2.getPeers() {
		if state := p.conn.GetState(); state != connectivity.Shutdown {
			t.Errorf("failed to leave network: %v ", state)
		}
	}
}

//func TestNode_GetPeers(t *testing.T) {
//	node1 := NewNode("localhost", 9390)
//	node2 := NewNode("localhost", 9389)
//	node1.StartServer()
//	defer node1.StopServer()
//
//	conn, err := node2.GetConnection(&node1.self)
//	if err != nil {
//		t.Fatal(err)
//	}
//	defer conn.Close()
//
//	client := NewNodeServiceClient(conn)
//	peers, err := client.GetPeers(context.Background(), &node2.self)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if len(peers.GetPeers()) != 0 && node1.getPeersNum() != 1 {
//		t.Fail()
//	}
//
//	peers, err = client.GetPeers(context.Background(), &node2.self)
//	if err != nil {
//		t.Fatal(err)
//	}
//	if len(peers.GetPeers()) != 1 && node1.getPeersNum() != 1 {
//		t.Fail()
//	}
//}
