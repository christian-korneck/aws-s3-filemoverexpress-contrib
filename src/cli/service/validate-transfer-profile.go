package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/awslabs/filemoverexpress/config"
	"github.com/awslabs/filemoverexpress/core/auth"
	transferapi "github.com/awslabs/filemoverexpress/core/transfer-api"
	"github.com/awslabs/filemoverexpress/logger"
	"github.com/awslabs/filemoverexpress/types/configtypes"
	"github.com/awslabs/filemoverexpress/types/pbtypes/fme/v1"
)

const (
	validateTransferProfileTimeout = 20 * time.Second
	// unsavedProfileName keys OIDC session lookups for a profile that has not been saved yet.
	unsavedProfileName = "unsaved-profile"
)

// newS3ManagerFunc is a variable so tests can simulate a slow session.
var newS3ManagerFunc = transferapi.NewS3Manager

// ValidateTransferProfile checks that the (possibly unsaved) transfer profile in the request can reach its
// bucket. Connection problems are reported in the response body rather than as RPC errors so that the GUI
// can show them inline next to its "Test connection" button.
func (*FileMoverServer) ValidateTransferProfile(
	ctx context.Context,
	req *connect.Request[fmev1.ValidateTransferProfileRequest],
) (*connect.Response[fmev1.ValidateTransferProfileResponse], error) {
	cfg := config.LoadConfiguration()
	if !isLocalClient(req.Peer()) && !cfg.APIServer.Permissions.AllowUIConfiguration {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New(strConfigurationDisabled))
	}
	if req.Msg.GetTransferProfile() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New(strValidateMissingProfile))
	}

	tp, err := profileToValidate(req.Msg.GetTransferProfile(), cfg.Protocols.S3.TransferProfiles)
	if err != nil {
		return connect.NewResponse(&fmev1.ValidateTransferProfileResponse{Message: err.Error()}), nil
	}

	ctx, cancel := context.WithTimeout(ctx, validateTransferProfileTimeout)
	defer cancel()

	message, ok := validateTransferProfile(ctx, tp)
	logger.Info(strValidateProfileResult, tp.Name, tp.Bucket, ok)

	return connect.NewResponse(&fmev1.ValidateTransferProfileResponse{
		Success: ok,
		Message: message,
	}), nil
}

// profileToValidate converts the request profile and applies what a saved profile would get: a name for
// OIDC session lookups, the provider's default region, and the stored secrets in place of placeholders.
func profileToValidate(pb *fmev1.TransferProfile, stored map[string]configtypes.TransferProfile) (configtypes.TransferProfile, error) {
	tp := configtypes.TransferProfileFromProtobuf(pb)
	if tp.Name == "" {
		tp.Name = unsavedProfileName
	}
	if tp.Region == "" {
		tp.Region = tp.Provider.Spec().DefaultRegion
	}
	err := tp.RestoreRedactedSecrets(stored)
	return tp, err
}

// validateTransferProfile builds an S3 client for the profile and performs the same ListObjectsV2 probe the
// CLI's validate-credentials command uses. It never includes credential material in the returned message.
func validateTransferProfile(ctx context.Context, tp configtypes.TransferProfile) (string, bool) {
	s3m, err := newS3ManagerContext(ctx, tp)
	if err != nil {
		if errors.Is(err, auth.ErrOIDCNotAuthenticated) {
			return strValidateSignInRequired, false
		}
		return fmt.Sprintf(strValidateSessionFailed, err), false
	}

	if err = s3m.ValidateCredentialsContext(ctx); err != nil {
		return fmt.Sprintf(strValidateListFailed, tp.Bucket, err), false
	}

	return fmt.Sprintf(strValidateProfileOK, tp.Bucket), true
}

// newS3ManagerContext bounds session creation (STS or OIDC token calls) by ctx. Those SDK paths do not take
// a context, so on timeout the creation is abandoned and finishes in the background.
func newS3ManagerContext(ctx context.Context, tp configtypes.TransferProfile) (*transferapi.S3Manager, error) {
	type result struct {
		s3m *transferapi.S3Manager
		err error
	}
	done := make(chan result, 1)
	go func() {
		s3m, err := newS3ManagerFunc(tp)
		done <- result{s3m, err}
	}()
	select {
	case r := <-done:
		return r.s3m, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
