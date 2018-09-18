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

	"google.golang.org/grpc/connectivity"
)

var tests = []string{
	"127.0.0.1:9388",
	"127.0.0.1:9389",
	"127.0.0.1:9390",
	"127.0.0.1:9391",
}

func TestPeerManager_AddPeers(t *testing.T) {
	pm := NewPeerManager("localhost:9488")
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
	pm := NewPeerManager("localhost:9488")
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
	pm := NewPeerManager("localhost:9488")

	node := NewNode(tests[0])
	node.StartServer()
	defer node.StopServer()

	conn, err := pm.GetConnection(tests[0])
	if err == nil {
		t.Errorf("get connection to unknown peer without error: %v", tests[0])
	}

	pm.AddPeers(tests[0])
	conn, err = pm.GetConnection(tests[0])
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()
	if state := conn.GetState(); state != connectivity.Idle {
		t.Errorf("failed to get connection to peer: %v %v(Expect IDLE)", tests[0], state)
	}
}

func TestPeerManager_Disconnect(t *testing.T) {
	pm := NewPeerManager("localhost:9488")

	node := NewNode(tests[0])
	node.StartServer()
	defer node.StopServer()

	if err := pm.Disconnect(tests[0]); err == nil {
		t.Errorf("disconnect to unknown peer without error: %v", tests[0])
	}

	pm.AddPeers(tests[0])
	_, err := pm.GetConnection(tests[0])
	if err != nil {
		t.Error(err)
	}
	if state := pm.Peers[tests[0]].conn.GetState(); state != connectivity.Idle {
		t.Errorf("failed to get connection to peer: %v %v(expect IDLE)", tests[0], state)
	}
	if err := pm.Disconnect(tests[0]); err != nil {
		t.Errorf("failed to disconnect peer: %v: %v", tests[0], err)
	}
	if state := pm.Peers[tests[0]].conn.GetState(); state != connectivity.Shutdown {
		t.Errorf("failed to disconnect peer: %v %v(expect SHUTDOWN)", tests[0], state)
	}
}

func TestPeerManager_RequestNeighbors(t *testing.T) {
	nodeAddr := "localhost:9488"
	pm := NewPeerManager("localhost:9588")
	pm.AddPeers(nodeAddr)

	node := NewNode(nodeAddr)
	node.StartServer()
	defer node.StopServer()

	peers, err := pm.RequestNeighbors(nodeAddr)
	if err != nil {
		t.Error(err)
	}
	if len(peers) != 0 {
		t.Errorf("failed to get peers from peer: %v %v(expect 0)", nodeAddr, len(peers))
	}

	node.RemovePeer(pm.self)
	node.AddPeers(tests...)
	peers, err = pm.RequestNeighbors(nodeAddr)
	if err != nil {
		t.Error(err)
	}
	if len(peers) != len(tests) {
		t.Errorf("failed to get peers from peer: %v %v(expect %v)", nodeAddr, len(peers), len(tests))
	}
}
