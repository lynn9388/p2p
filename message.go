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

// SendMessage sends message to a peer through a connection.
func SendMessage(conn *grpc.ClientConn, ctx context.Context, msg proto.Message, timeout time.Duration) (*any.Any, error) {
	client := NewMessageServiceClient(conn)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	anyMsg, err := ptypes.MarshalAny(msg)
	if err != nil {
		return nil, err
	}

	return client.ReceiveMessage(ctx, anyMsg)
}
