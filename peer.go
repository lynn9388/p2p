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
	"github.com/dedis/student_18/dgcosi/code/onet/log"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// Peer is the the remote node that a local node can connect to.
type Peer struct {
	Addr string
	conn *grpc.ClientConn
}

// GetConnection returns a connection to the peer.
func (p *Peer) GetConnection() (*grpc.ClientConn, error) {
	if p.conn == nil || p.conn.GetState() != connectivity.Idle &&
		p.conn.GetState() != connectivity.Ready {
		if p.conn != nil {
			if err := p.Disconnect(); err != nil {
				log.Print(err)
			}
		}
		conn, err := grpc.Dial(p.Addr, grpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		p.conn = conn
		log.Printf("connected: %v", p.Addr)
	}

	return p.conn, nil
}

// Disconnect closes the connection to the peer.
func (p *Peer) Disconnect() error {
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			return errors.New("failed to close connection: " + err.Error())
		}
		log.Printf("closed: %v", p.Addr)
	}
	return nil
}

// GetPeers requests other neighbor peers from the remote peer.
func (p *Peer) GetPeers(addr string) ([]string, error) {
	conn, err := p.GetConnection()
	if err != nil {
		return nil, err
	}
	client := NewNodeServiceClient(conn)
	peers, err := client.GetPeers(context.Background(), &wrappers.StringValue{Value: addr})
	if err != nil {
		return nil, errors.New("failed to get peers: " + err.Error())
	}
	return peers.Peers, nil
}
