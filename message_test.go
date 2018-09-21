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
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"
	"github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"
)

func TestSendMessage(t *testing.T) {
	sendMessage := "lynn"
	replyMessage := "9388"

	checkStringMessage := func(ctx context.Context, msg *any.Any) (*any.Any, error) {
		var err error
		m := &wrappers.StringValue{}
		if err = ptypes.UnmarshalAny(msg, m); err != nil {
			t.Error(err)
		}

		if m.Value != sendMessage {
			t.Errorf("faild to receive message: %v", m.Value)
		}

		reply, err := ptypes.MarshalAny(&wrappers.StringValue{Value: replyMessage})
		if err != nil {
			t.Error(err)
		}

		return reply, err
	}

	server := NewNode(tests[0])
	server.MessageManager.RegisterProcess(&wrappers.StringValue{}, checkStringMessage)
	server.StartServer()
	defer server.StopServer()

	conn, err := grpc.Dial(server.Addr, grpc.WithInsecure())
	if err != nil {
		t.Error(err)
	}
	defer conn.Close()

	reply, err := SendMessage(conn, context.Background(), &wrappers.StringValue{Value: sendMessage}, 1*time.Second)
	if err != nil {
		t.Error(err)
	}

	m := &wrappers.StringValue{}
	if err = ptypes.UnmarshalAny(reply, m); err != nil {
		t.Error(err)
	}

	if m.Value != replyMessage {
		t.Errorf("faild to receive reply: %v", m.Value)
	}
}
