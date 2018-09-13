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
	"127.0.0.1:9389",
}

func TestNode_AddPeers(t *testing.T) {
	node := NewNode("localhost:9488")
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
	node := NewNode("localhost:9488")

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
	for _, addr := range tests {
		node := NewNode(addr)
		node.JoinNetwork(tests[0])
		defer node.LeaveNetwork()
		node.StartServer()
		defer node.StopServer()
	}

	node := NewNode("localhost:9488")
	node.JoinNetwork(tests[0])
	defer node.LeaveNetwork()
	node.StartServer()
	defer node.StopServer()
	time.Sleep(5 * time.Second)
	if node.getPeersNum() != len(tests) {
		t.Errorf("failed to join the network (expect %v): %v", len(tests), node.getPeersNum())
	}
}

func TestNode_LeaveNetwork(t *testing.T) {
	for _, addr := range tests {
		node := NewNode(addr)
		node.JoinNetwork(tests[0])
		defer node.LeaveNetwork()
		node.StartServer()
		defer node.StopServer()
	}

	node := NewNode("localhost:9488")
	node.JoinNetwork(tests[0])
	node.StartServer()
	defer node.StopServer()
	time.Sleep(5 * time.Second)

	node.LeaveNetwork()
	for _, p := range node.getPeers() {
		if state := p.conn.GetState(); state != connectivity.Shutdown {
			t.Errorf("failed to leave network: %v ", state)
		}
	}
}
