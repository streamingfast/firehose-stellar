package rpc

import (
	"github.com/streamingfast/jsonrpc"
)

type Client struct {
	rpc *jsonrpc.RPCClient
}

func NewClient(endpoint string) *Client {
	return &Client{
		rpc: jsonrpc.NewRPCClient(endpoint),
	}
}

//todo: add func getTransaction ...
