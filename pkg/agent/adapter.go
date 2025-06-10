package agent

import (
	tunnelv1 "github.com/xuezhaojun/multiclustertunnel/api/api/v1"
)

// PacketStream is a bidirectional channel of packets for a single stream.
type PacketStream interface {
	Recv() <-chan *tunnelv1.Packet
	Send(*tunnelv1.Packet) error
	StreamID() int64
	Close() error
}

// ProxyAdapter is the interface that developers implement to create their ServiceProxy logic.
type ProxyAdapter interface {
	// AdaptAndForward is called by the Agent when it needs to create a new outbound connection based on the instructions from the Hub.
	// It returns a PacketStream that the agent core will use to proxy data.
	AdaptAndForward(dialPacket *tunnelv1.Packet) (PacketStream, error)
}
