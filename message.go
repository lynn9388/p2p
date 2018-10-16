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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Process is the function to process one type of message.
type Process func(*any.Any) (*any.Any, error)

// messageLog is a log item for a message. Only one of sender and receiver
// need to be assigned.
type messageLog struct {
	hash     string
	sender   string
	receiver string
	time     time.Time
}

// MessageManager is the service to receive and process messages.
type MessageManager struct {
	ProcessSet  map[string]Process // process for every message type
	MessageLogs []messageLog       // logs for sent/received messages
}

// NewMessageManager returns a initialized message manager.
func NewMessageManager() *MessageManager {
	return &MessageManager{
		ProcessSet:  make(map[string]Process, 0),
		MessageLogs: make([]messageLog, 0),
	}
}

// RegisterProcess registers a process for a type of message.
func (mm *MessageManager) RegisterProcess(x proto.Message, p Process) {
	name := proto.MessageName(x)
	mm.ProcessSet[name] = p
}

// SendMessage sends message to a peer through a connection.
func (mm *MessageManager) SendMessage(ctx context.Context, sender string, conn *grpc.ClientConn, msg proto.Message, timeout time.Duration) (*any.Any, error) {
	anyMsg, err := ptypes.MarshalAny(msg)
	if err != nil {
		return nil, err
	}

	ctx = newContextWithSender(ctx, sender)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	mm.MessageLogs = append(mm.MessageLogs, messageLog{hash: hash(anyMsg.Value), receiver: conn.Target(), time: time.Now()})
	return NewMessageServiceClient(conn).ReceiveMessage(ctx, anyMsg)
}

// ReceiveMessage receives message from a peer and process it.
func (mm *MessageManager) ReceiveMessage(ctx context.Context, msg *any.Any) (*any.Any, error) {
	sender, err := getSender(ctx)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	mm.MessageLogs = append(mm.MessageLogs, messageLog{hash: hash(msg.Value), sender: sender, time: time.Now()})

	p, err := mm.getProcess(msg)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return p(msg)
}

// getProcess returns the process function for a message type.
func (mm *MessageManager) getProcess(msg *any.Any) (Process, error) {
	name := path.Base(msg.TypeUrl)
	p, ok := mm.ProcessSet[name]
	if !ok {
		return nil, fmt.Errorf("failed to find process for message type: %v", name)
	}
	return p, nil
}

// hash returns the hash value of data.
func hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// newContextWithSender returns a context with a value of sender's address.
func newContextWithSender(ctx context.Context, sender string) context.Context {
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("sender", sender))
}

// getSender returns the sender's address in context if it exists.
func getSender(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("failed to get metadata from context")
	}

	sender := md.Get("sender")
	if len(sender) != 1 {
		return "", errors.New("failed to get address of message sender from context")
	}

	return sender[0], nil
}
