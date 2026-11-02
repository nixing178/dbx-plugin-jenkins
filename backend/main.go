package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/nixing178/dbx-plugin-jenkins/internal/rpc"
	dbxpluginsdk "github.com/t8y2/dbx/plugins/sdk/go/dbx-plugin-sdk"
)

type plugin struct {
	service *rpc.Service
}

func (p *plugin) Handle(
	_ dbxpluginsdk.RequestContext,
	method string,
	params json.RawMessage,
	_ *dbxpluginsdk.Emitter,
) (any, *dbxpluginsdk.PluginError) {
	result, rpcErr := p.service.Handle(context.Background(), method, params)
	if rpcErr != nil {
		return nil, dbxpluginsdk.NewError(rpcErr.Code, rpcErr.Message)
	}
	return result, nil
}

func main() {
	metadata := dbxpluginsdk.Metadata{
		ID:           "io.github.nixing178.jenkins",
		Version:      "0.11.0",
		Capabilities: []string{"connections"},
	}
	server := dbxpluginsdk.NewServer(metadata, &plugin{service: rpc.NewService()})
	if err := server.Serve(); err != nil {
		log.Fatal(err)
	}
}
