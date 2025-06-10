package hub

import (
	tunnelv1 "github.com/xuezhaojun/multiclustertunnel/api/api/v1"
)

// PacketStream is a bidirectional channel of packets for a single stream.
// This is the primary interface for an adapter to interact with a stream.
type PacketStream interface {
	// Recv returns a channel to receive packets from the agent.
	Recv() <-chan *tunnelv1.Packet
	// Send sends a packet to the agent.
	Send(*tunnelv1.Packet) error
	// StreamID returns the stream ID.
	StreamID() int64
	// Close closes the stream.
	Close() error
}

// HubAdapter is the interface that developers implement to create their HubGateway logic.
// It dictates how new streams are handled.
type HubAdapter interface {
	// ServeStream is called by the Hub whenever a new stream is initiated by the ServiceProxy (e.g., via the first packet).
	// The implementation should handle the entire lifecycle of this stream.
	// This method should run in its own goroutine and exit when the stream is done.
	ServeStream(stream PacketStream)
}
