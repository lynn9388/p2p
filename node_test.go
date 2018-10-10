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
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/wrappers"
)

func TestNode_SendMessage(t *testing.T) {
	sender := NewNode(tests[0])
	sender.StartServer()
	defer sender.StopServer()

	receiver := NewNode(tests[1])
	receiver.RegisterProcess(&wrappers.StringValue{}, checkStringMessage)
	receiver.StartServer()
	defer receiver.StopServer()

	waiter := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		waiter.Add(1)
		go func() {
			reply, err := sender.SendMessage(receiver.Addr, &wrappers.StringValue{Value: testSendMsg}, 10*time.Second)
			if err != nil {
				t.Error(err)
				waiter.Done()
				return
			}

			m := &wrappers.StringValue{}
			if err = ptypes.UnmarshalAny(reply, m); err != nil {
				t.Error(err)
				waiter.Done()
				return
			}

			if m.Value != testReplyMsg {
				t.Errorf("faild to receive reply: %v", m.Value)
			}
			waiter.Done()
		}()
	}
	waiter.Wait()
}

func TestNode_Broadcast(t *testing.T) {
	var nodes []*Node
	for _, addr := range tests {
		node := NewNode(addr)
		node.RegisterProcess(&wrappers.StringValue{}, checkStringMessage)
		node.StartServer()
		defer node.StopServer()
		nodes = append(nodes, node)
	}

	nodes[0].PeerManager.AddPeers(nodes[1].Addr, nodes[2].Addr)
	nodes[1].PeerManager.AddPeers(nodes[0].Addr, nodes[3].Addr)
	nodes[2].PeerManager.AddPeers(nodes[0].Addr, nodes[3].Addr)
	nodes[3].PeerManager.AddPeers(nodes[1].Addr, nodes[2].Addr)

	if err := nodes[0].Broadcast(&wrappers.StringValue{Value: testSendMsg}, 10*time.Second); err != nil {
		t.Error(err)
	}
	time.Sleep(1 * time.Second)

	for _, node := range nodes {
		if len(node.MessageLog) != 2 {
			t.Errorf("%v failed to broadcast message: %v(expecte 2)", node.Addr, len(node.MessageLog))
		}
	}
}
