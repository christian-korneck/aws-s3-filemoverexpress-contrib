package service

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/awslabs/filemoverexpress/config"
	"github.com/awslabs/filemoverexpress/types/configtypes"
	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
)

func (*FileMoverServer) SetConfiguration(
	_ context.Context,
	req *connect.Request[fmev1.GRPCFmeConfig],
) (*connect.Response[fmev1.SetConfigurationResponse], error) {
	cfg := config.LoadConfiguration()
	allowConfigEdit := cfg.APIServer.Permissions.AllowUIConfiguration
	if !isLocalClient(req.Peer()) && !allowConfigEdit {
		return connect.NewResponse(&fmev1.SetConfigurationResponse{
			Success: false,
			Message: strConfigurationDisabled,
		}), connect.NewError(connect.CodeResourceExhausted, errors.New(strUnableToWriteConfig))
	}

	newConfig, err := configtypes.FromGRPCProtobuf(req.Msg, cfg.APIServer)
	if err != nil {
		return connect.NewResponse(&fmev1.SetConfigurationResponse{
			Success: false,
			Message: err.Error(),
		}), connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err = restoreRedactedSecrets(&newConfig, cfg.Protocols.S3.TransferProfiles); err != nil {
		return connect.NewResponse(&fmev1.SetConfigurationResponse{
			Success: false,
			Message: err.Error(),
		}), connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = config.SaveConfig(&newConfig)
	if err != nil {
		return connect.NewResponse(&fmev1.SetConfigurationResponse{
			Success: false,
			Message: strUnableToWriteConfig,
		}), connect.NewError(connect.CodeInternal, errors.New(strUnableToWriteConfig))
	}

	return connect.NewResponse(&fmev1.SetConfigurationResponse{
		Success: true,
		Message: strSuccessFullyUpdatedConfig,
	}), nil
}

// restoreRedactedSecrets puts the stored secrets back into every profile that the client returned with the
// redaction placeholder, because GetConfiguration never sends them.
func restoreRedactedSecrets(newConfig *configtypes.FmeConfig, stored map[string]configtypes.TransferProfile) error {
	for name, transferProfile := range newConfig.Protocols.S3.TransferProfiles {
		if err := transferProfile.RestoreRedactedSecrets(stored); err != nil {
			return err
		}
		newConfig.Protocols.S3.TransferProfiles[name] = transferProfile
	}
	return nil
}
