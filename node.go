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
	"google.golang.org/grpc/metadata"
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
	return n.MessageManager.SendMessage(context.Background(), conn, msg, timeout)
}

// Broadcast sends a broadcast message to the neighbor peers.
func (n *Node) Broadcast(msg proto.Message, timeout time.Duration) error {
	anyMsg, err := ptypes.MarshalAny(msg)
	if err != nil {
		return err
	}

	var buff bytes.Buffer
	var wg sync.WaitGroup
	for _, addr := range n.PeerManager.GetPeers() {
		wg.Add(1)
		go func(addr string) {
			if err := n.SendBroadcast(addr, anyMsg, timeout); err != nil {
				log.Errorf("failed to broadcast message to peer: %v : %v", addr, err)
				buff.WriteString(err.Error() + "\n")
			}
			wg.Done()
		}(addr)
	}
	wg.Wait()
	if buff.Len() != 0 {
		return errors.New(strings.TrimSpace(buff.String()))
	}
	return nil
}

// SendBroadcast sends a broadcast message to a peers.
func (n *Node) SendBroadcast(addr string, msg *any.Any, timeout time.Duration) error {
	conn, err := n.PeerManager.GetConnection(addr)
	if err != nil {
		return err
	}

	client := NewNodeServiceClient(conn)

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("address", n.Addr))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	n.MessageLog = append(n.MessageLog, hashLog{hash: hash(msg.Value), time: time.Now()})
	_, err = client.ReceiveBroadcast(ctx, msg)
	return err
}

// ReceiveBroadcast receives message and relay message to neighbor peers.
// The node will not broadcast messages with same content within 1 minutes.
func (n *Node) ReceiveBroadcast(ctx context.Context, msg *any.Any) (*any.Any, error) {
	printError := func(err error) error {
		log.Error(err)
		return err
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, printError(errors.New("failed to get metadata from context"))
	}
	addresses := md.Get("address")
	if len(addresses) != 1 {
		return nil, printError(errors.New("failed to get address of message sender from context"))
	}

	hash := hash(msg.Value)
	now := time.Now()
	for i := len(n.MessageLog) - 1; i >= 0; i-- {
		if n.MessageLog[i].time.Add(1 * time.Minute).Before(now) {
			break
		}

		if n.MessageLog[i].hash == hash {
			err := fmt.Errorf("%v received duplicate message from peer: %v", n.Addr, addresses[0])
			log.Debug(err)
			return &any.Any{}, nil
		}
	}

	name := path.Base(msg.TypeUrl)
	p, ok := n.ProcessSet[name]
	if !ok {
		return nil, printError(fmt.Errorf("failed to find process for message type: %v", name))
	}
	n.MessageLog = append(n.MessageLog, hashLog{hash: hash, time: now})

	log.Debugf("%v received broadcast message from peer: %v", n.Addr, addresses[0])

	for _, addr := range n.PeerManager.GetPeers() {
		if addr != addresses[0] {
			go func() {
				if err := n.SendBroadcast(addr, msg, 10*time.Second); err != nil {
					log.Error(err)
				}
			}()
		}
	}

	return p(ctx, msg)
}
