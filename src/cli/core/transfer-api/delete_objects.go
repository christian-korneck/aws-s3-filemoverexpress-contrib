package transfer_api

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/awslabs/filemoverexpress/events"
	fterrors "github.com/awslabs/filemoverexpress/fme-errors"
	"github.com/awslabs/filemoverexpress/logger"
)

const (
	// deleteScopePrefix deletes everything under a prefix; deleteScopeExactKey deletes a single key.
	deleteScopePrefix deleteScope = iota
	deleteScopeExactKey

	MaxObjectsPerDeleteObjectsCall = 1000 // defined by AWS SDK DeleteObjects()
)

// deleteScope selects whether a deletion listing targets a whole prefix or exactly one key.
type deleteScope int

func (s3m *S3Manager) DeleteObject(objectPrefix string) error {
	cleanedObjectPrefix := FormatAsS3Object(objectPrefix)
	events.Events.Info("Started deletion of %s from bucket %s", cleanedObjectPrefix, s3m.Bucket)

	// check if path is empty
	versionsOutput, err := s3m.listDeletable(objectPrefix, deleteScopeExactKey)
	if err != nil {
		events.Events.Error("%s: %s", fterrors.ErrGetObjectVersions.Error(), err.Error())
		return fmt.Errorf("%w: %w", fterrors.ErrGetObjectVersions, err)
	}
	if len(versionsOutput.S3ObjectVersions) == 0 {
		events.Events.Error("Error deleting %s from bucket %s: %s", objectPrefix, s3m.Bucket,
			fterrors.ErrNoObjectVersionsToDelete.Error())
		return fmt.Errorf("error deleting %s: %w", objectPrefix, fterrors.ErrNoObjectVersionsToDelete)
	}

	return s3m.deleteVersions(versionsOutput.S3ObjectVersions, objectPrefix)
}

func (s3m *S3Manager) DeletePrefix(prefixToDelete string) error {
	cleanedPrefixToDelete := FormatAsS3Prefix(prefixToDelete)
	events.Events.Info("Started deletion of %s from bucket %s", cleanedPrefixToDelete, s3m.Bucket)

	// check if path is empty
	if cleanedPrefixToDelete == "" || cleanedPrefixToDelete == "/" {
		events.Events.Error("Error deleting %s from bucket %s: %s", cleanedPrefixToDelete, s3m.Bucket,
			fterrors.ErrDeleteEmptyPath.Error())
		return fterrors.ErrDeleteEmptyPath
	}

	// discover versions
	versionsOutput, err := s3m.listDeletable(cleanedPrefixToDelete, deleteScopePrefix)
	if err != nil {
		events.Events.Error("%s: %s", fterrors.ErrGetObjectVersions.Error(), err.Error())
		return fmt.Errorf("%w: %w", fterrors.ErrGetObjectVersions, err)
	}
	if len(versionsOutput.S3ObjectVersions) == 0 {
		events.Events.Error("Error deleting %s from bucket %s: %s", cleanedPrefixToDelete, s3m.Bucket,
			fterrors.ErrNoObjectVersionsToDelete.Error())
		return fmt.Errorf("error deleting %s: %w", cleanedPrefixToDelete, fterrors.ErrNoObjectVersionsToDelete)
	}

	return s3m.deleteVersions(versionsOutput.S3ObjectVersions, cleanedPrefixToDelete)
}

// listDeletable lists everything that must be removed to delete a path: every version and delete marker
// where the provider supports versioning, otherwise (or when the endpoint rejects ListObjectVersions) the
// current objects by key.
func (s3m *S3Manager) listDeletable(path string, scope deleteScope) (ListObjectVersionsOutput, error) {
	if !s3m.Provider.Spec().SupportsVersioning {
		return s3m.listUnversioned(path, scope)
	}

	output, err := s3m.listVersions(path, scope)
	if err != nil && isUnsupportedOperation(err) {
		logger.Warn(strVersioningUnsupportedFallback, s3m.Bucket, err)
		return s3m.listUnversioned(path, scope)
	}
	return output, err
}

func (s3m *S3Manager) listVersions(path string, scope deleteScope) (ListObjectVersionsOutput, error) {
	if scope == deleteScopeExactKey {
		return s3m.ListVersionsForObject(path)
	}
	return s3m.ListVersionsInPrefix(path)
}

func (s3m *S3Manager) deleteVersions(versionsToDelete []S3ObjectVersion, pathToDelete string) error {
	// call DeleteObjects on discovered objects MaxObjectsPerDeleteObjectsCall at a time
	numDeleteErrors := 0
	for start := 0; start < len(versionsToDelete); start += MaxObjectsPerDeleteObjectsCall {
		end := min(start+MaxObjectsPerDeleteObjectsCall, len(versionsToDelete))
		failures, err := s3m.deleteBatch(toObjectIdentifiers(versionsToDelete[start:end]))
		numDeleteErrors += failures
		if err != nil {
			return err
		}
	}

	if numDeleteErrors > 0 {
		events.Events.Error("%d error(s) while deleting %s from bucket %s: %s", numDeleteErrors,
			pathToDelete, s3m.Bucket, fterrors.ErrDeleteErrorsOccurred.Error())
		return fmt.Errorf("%d error(s) while deleting %s from bucket %s: %w", numDeleteErrors,
			pathToDelete, s3m.Bucket, fterrors.ErrDeleteErrorsOccurred)
	}

	events.Events.Info("Successfully completed deletion of %s from bucket %s", pathToDelete, s3m.Bucket)
	return nil
}

// toObjectIdentifiers converts listed versions to delete identifiers. A VersionId is only sent when one is
// known; unversioned listings leave it empty so the request stays valid for stores without versioning.
func toObjectIdentifiers(versions []S3ObjectVersion) []types.ObjectIdentifier {
	identifiers := make([]types.ObjectIdentifier, len(versions))
	for i := range versions {
		identifiers[i] = types.ObjectIdentifier{Key: &versions[i].Key}
		if versions[i].VersionId != "" {
			identifiers[i].VersionId = &versions[i].VersionId
		}
	}
	return identifiers
}

// deleteBatch issues one DeleteObjects call and returns the number of per-object failures it reported. When
// the endpoint does not implement batch deletion it falls back to deleting the objects one at a time.
func (s3m *S3Manager) deleteBatch(objects []types.ObjectIdentifier) (int, error) {
	output, err := s3m.Client.DeleteObjects(context.TODO(), &s3.DeleteObjectsInput{
		Bucket: &s3m.Bucket,
		Delete: &types.Delete{Objects: objects},
	})
	if err != nil && isBatchDeleteRejected(err) {
		logger.Warn(strBatchDeleteUnsupportedFallback, s3m.Bucket, err)
		return s3m.deleteOneByOne(objects), nil
	}

	failures := 0
	if output != nil {
		for _, success := range output.Deleted {
			logger.Info("Successfully deleted %s version %s from bucket %s", aws.ToString(success.Key),
				aws.ToString(success.VersionId), s3m.Bucket)
		}
		for _, failure := range output.Errors {
			failures++
			events.Events.Error("Error deleting %s version %s from bucket %s: %s", aws.ToString(failure.Key),
				aws.ToString(failure.VersionId), s3m.Bucket, aws.ToString(failure.Message))
		}
	}
	return failures, err
}

// deleteOneByOne deletes each object individually and returns the number of failures.
func (s3m *S3Manager) deleteOneByOne(objects []types.ObjectIdentifier) int {
	failures := 0
	for _, object := range objects {
		_, err := s3m.Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket:    &s3m.Bucket,
			Key:       object.Key,
			VersionId: object.VersionId,
		})
		if err != nil {
			failures++
			events.Events.Error(strDeleteObjectFailed, aws.ToString(object.Key), s3m.Bucket, err)
			continue
		}
		logger.Info(strDeleteObjectSucceeded, aws.ToString(object.Key), s3m.Bucket)
	}
	return failures
}
