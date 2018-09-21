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

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
)

func TestNode_RequestBroadcast(t *testing.T) {
	var nodes []*Node
	for _, addr := range tests {
		node := NewNode(addr)
		node.StartServer()
		defer node.StopServer()
		nodes = append(nodes, node)
	}

	nodes[0].PeerManager.AddPeers(nodes[1].Addr, nodes[2].Addr)
	nodes[1].PeerManager.AddPeers(nodes[0].Addr, nodes[3].Addr)
	nodes[2].PeerManager.AddPeers(nodes[0].Addr, nodes[3].Addr)
	nodes[3].PeerManager.AddPeers(nodes[1].Addr, nodes[2].Addr)

	msg, err := ptypes.MarshalAny(&wrappers.StringValue{Value: "Hello"})
	if err != nil {
		t.Error(err)
	}

	if err = nodes[0].RequestBroadcast(nodes[1].Addr, msg); err != nil {
		t.Error(err)
	}
	time.Sleep(1 * time.Second)

	for _, node := range nodes {
		length := 0
		node.messages.Range(func(key, value interface{}) bool {
			length++
			return true
		})
		if length != 1 {
			t.Errorf("failed to broadcast message: %v(expecte 1)", length)
		}
	}
}
