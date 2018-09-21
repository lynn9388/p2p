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
	"fmt"
	"path"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc"
)

// Process is the function to process one type of message.
type Process func(context.Context, *any.Any) (*any.Any, error)

// MessageManager is the service to receive and process messages.
type MessageManager struct {
	ProcessSet map[string]Process
}

// NewMessageManager returns a initialized message manager.
func NewMessageManager() *MessageManager {
	return &MessageManager{
		ProcessSet: make(map[string]Process, 0),
	}
}

// RegisterProcess registers a process for a type of message.
func (mm *MessageManager) RegisterProcess(x proto.Message, p Process) {
	name := proto.MessageName(x)
	mm.ProcessSet[name] = p
}

// ReceiveMessage receives message from a peer and process it.
func (mm *MessageManager) ReceiveMessage(ctx context.Context, msg *any.Any) (*any.Any, error) {
	name := path.Base(msg.TypeUrl)
	p, ok := mm.ProcessSet[name]
	if !ok {
		return nil, fmt.Errorf("failed to find process for message type: %v", name)
	}

	return p(ctx, msg)
}

// sendMessage sends message to a peer through a connection.
func sendMessage(conn *grpc.ClientConn, ctx context.Context, msg proto.Message, timeout time.Duration) (*any.Any, error) {
	client := NewMessageServiceClient(conn)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	anyMsg, err := ptypes.MarshalAny(msg)
	if err != nil {
		return nil, err
	}

	return client.ReceiveMessage(ctx, anyMsg)
}
