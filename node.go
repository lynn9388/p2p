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

// Package p2p implements a node in P2P network.
package p2p

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Node is a independent entity in the P2P network.
type Node struct {
	Addr            string         // network address
	Server          *grpc.Server   // gRPC server
	Waiter          sync.WaitGroup // wait server running in background
	PeerManager     *PeerManager   // peer manager
	*MessageManager                // message manager
}

var (
	log *zap.SugaredLogger // default logger
)

func init() {
	logger, _ := zap.NewDevelopment()
	log = logger.Sugar()
}

// NewNode initials a new node with specific network address.
func NewNode(addr string) *Node {
	return &Node{
		Addr:           addr,
		Server:         grpc.NewServer(),
		PeerManager:    NewPeerManager(addr),
		MessageManager: NewMessageManager(),
	}
}

// StartServer starts server to provide services. This must be called after
// registering any other external service.
func (n *Node) StartServer() {
	conn, _ := net.DialTimeout("tcp", n.Addr, 5*time.Second)
	if conn != nil {
		conn.Close()
	}

	lis, err := net.Listen("tcp", n.Addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// register internal service
	RegisterPeerServiceServer(n.Server, n.PeerManager)
	RegisterMessageServiceServer(n.Server, n.MessageManager)
	RegisterNodeServiceServer(n.Server, n)

	log.Infof("server is listening at: %v", n.Addr)
	n.Waiter.Add(1)
	go n.Server.Serve(lis)
}

// StopServer stops the server.
func (n *Node) StopServer() {
	if n.Server != nil {
		n.Server.Stop()
		log.Infof("server stopped: %v", n.Addr)
		n.Waiter.Done()
	}
}

// Wait keeps the server of the node running in background.
func (n *Node) Wait() {
	n.Waiter.Wait()
}

// SendMessage sends a message to a peer.
func (n *Node) SendMessage(addr string, msg proto.Message, timeout time.Duration) (*any.Any, error) {
	conn, err := n.PeerManager.GetConnection(addr)
	if err != nil {
		return nil, err
	}
	return n.MessageManager.SendMessage(context.Background(), n.Addr, conn, msg, timeout)
}

// broadcast sends a broadcast to neighbor peers.
func (n *Node) broadcast(ctx context.Context, msg *any.Any, timeout time.Duration) error {
	var buff bytes.Buffer
	var wg sync.WaitGroup
	for _, addr := range n.PeerManager.GetPeers() {
		if n.existRelevantMessage(msg, addr) {
			continue
		}

		wg.Add(1)
		go func(addr string) {
			defer wg.Done()

			conn, err := n.PeerManager.GetConnection(addr)
			if err != nil {
				buff.WriteString(err.Error() + "\n")
				return
			}

			ctx = newContextWithSender(ctx, n.Addr)
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			n.MessageLogs = append(n.MessageLogs, messageLog{hash: hash(msg.Value), receiver: addr, time: time.Now()})
			_, err = NewNodeServiceClient(conn).ReceiveBroadcast(ctx, msg)
			if err != nil {
				buff.WriteString(err.Error() + "\n")
			}
		}(addr)
	}
	wg.Wait()
	if buff.Len() != 0 {
		return errors.New(strings.TrimSpace(buff.String()))
	}
	return nil
}

// Broadcast sends a broadcast message to neighbor peers.
func (n *Node) Broadcast(msg proto.Message, timeout time.Duration) error {
	anyMsg, err := ptypes.MarshalAny(msg)
	if err != nil {
		return err
	}
	return n.broadcast(context.Background(), anyMsg, timeout)
}

// ReceiveBroadcast receives message and relay message to neighbor peers.
// The node will not broadcast messages with same content within 1 minutes.
func (n *Node) ReceiveBroadcast(ctx context.Context, msg *any.Any) (*any.Any, error) {
	sender, err := getSender(ctx)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if n.existRelevantMessage(msg, "") {
		err := fmt.Errorf("%v received duplicate message from peer: %v", n.Addr, sender)
		log.Debug(err)
		return &any.Any{}, nil
	}

	name := path.Base(msg.TypeUrl)
	p, ok := n.ProcessSet[name]
	if !ok {
		return nil, fmt.Errorf("failed to find process for message type: %v", name)
	}
	n.MessageLogs = append(n.MessageLogs, messageLog{hash: hash(msg.Value), sender: sender, time: time.Now()})

	log.Debugf("%v received broadcast message from peer: %v", n.Addr, sender)

	err = n.broadcast(context.Background(), msg, 10*time.Second)
	if err != nil {
		log.Error(err)
	}

	return p(msg)
}

// existRelevantMessage checks if a message log's sender or receivers is
// equal to an address.
func (n *Node) existRelevantMessage(msg *any.Any, addr string) bool {
	hash := hash(msg.Value)
	timeline := time.Now().Add(-1 * time.Minute)
	for i := len(n.MessageLogs) - 1; i >= 0; i-- {
		log := n.MessageLogs[i]

		if log.time.Before(timeline) {
			break
		}

		if log.hash == hash && log.sender == addr || log.receiver == addr {
			return true
		}
	}
	return false
}
