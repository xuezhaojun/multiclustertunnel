package server

import (
	"net/http"
)

type Router interface {
	// Parse extracts the target cluster name and target address from the HTTP request
	// This allows developers to implement custom routing logic based on path, headers, or other request attributes
	// cluster: the name of the managed cluster to route the request to
	// targetAddress: the target service address within the managed cluster (will be set in packet.TargetAddress)
	Parse(r *http.Request) (cluster, targetAddress string, err error)
}
