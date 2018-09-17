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
	"testing"
	"time"

	"google.golang.org/grpc/connectivity"
)

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
