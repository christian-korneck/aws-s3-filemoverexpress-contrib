package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/awslabs/filemoverexpress/config"
	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
)

func (*FileMoverServer) GetConfiguration(
	_ context.Context,
	req *connect.Request[fmev1.GetConfigurationRequest],
) (*connect.Response[fmev1.GRPCFmeConfig], error) {
	cfg := config.LoadConfiguration()
	allowConfigEdit := cfg.APIServer.Permissions.AllowUIConfiguration
	if !isLocalClient(req.Peer()) && !allowConfigEdit {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New(strConfigurationDisabled))
	}

	// Stored secrets never leave the daemon; SetConfiguration restores them when the placeholder comes back.
	return connect.NewResponse(cfg.Redacted().ToGRPCProtobuf()), nil
}
