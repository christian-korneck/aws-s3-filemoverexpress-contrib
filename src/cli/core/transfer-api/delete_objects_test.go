package transfer_api

import (
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/awslabs/filemoverexpress/types/configtypes"
)

// deleteCapturingClient implements the parts of FileMoverS3ClientInterface used by DeleteObject and
// DeletePrefix. Only the configured behaviours are implemented; the embedded interface is nil so any
// unexpected call panics and surfaces as a test failure.
type deleteCapturingClient struct {
	FileMoverS3ClientInterface

	objects          []s3types.Object
	versions         []s3types.ObjectVersion
	listVersionsErr  error
	deleteObjectsErr error

	listVersionsCalls   int
	batchIdentifiers    []s3types.ObjectIdentifier
	singleDeletedKeys   []string
	deleteObjectsCalled int
}

func (c *deleteCapturingClient) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input,
	_ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{Contents: c.objects, IsTruncated: aws.Bool(false)}, nil
}

func (c *deleteCapturingClient) ListObjectVersions(_ context.Context, _ *s3.ListObjectVersionsInput,
	_ ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error) {
	c.listVersionsCalls++
	if c.listVersionsErr != nil {
		return nil, c.listVersionsErr
	}
	return &s3.ListObjectVersionsOutput{Versions: c.versions, IsTruncated: aws.Bool(false)}, nil
}

func (c *deleteCapturingClient) DeleteObjects(_ context.Context, params *s3.DeleteObjectsInput,
	_ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	c.deleteObjectsCalled++
	if c.deleteObjectsErr != nil {
		return nil, c.deleteObjectsErr
	}
	c.batchIdentifiers = append(c.batchIdentifiers, params.Delete.Objects...)
	deleted := make([]s3types.DeletedObject, 0, len(params.Delete.Objects))
	for _, obj := range params.Delete.Objects {
		deleted = append(deleted, s3types.DeletedObject{Key: obj.Key, VersionId: obj.VersionId})
	}
	return &s3.DeleteObjectsOutput{Deleted: deleted}, nil
}

func (c *deleteCapturingClient) DeleteObject(_ context.Context, params *s3.DeleteObjectInput,
	_ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	c.singleDeletedKeys = append(c.singleDeletedKeys, aws.ToString(params.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func newDeleteTestManager(client FileMoverS3ClientInterface, provider configtypes.S3Provider) *S3Manager {
	return &S3Manager{
		Bucket:   "bucket",
		Client:   client,
		Lock:     &sync.RWMutex{},
		Provider: provider,
	}
}

var notImplemented = &smithy.GenericAPIError{Code: "NotImplemented", Message: "ListObjectVersions is not implemented"}

func twoObjects() []s3types.Object {
	return []s3types.Object{
		{Key: aws.String("media/a.mov"), Size: aws.Int64(10)},
		{Key: aws.String("media/b.mov"), Size: aws.Int64(20)},
	}
}

func TestDeletePrefix_FallsBackWhenVersioningUnsupported(t *testing.T) {
	client := &deleteCapturingClient{objects: twoObjects(), listVersionsErr: notImplemented}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderMinIO)

	if err := s3m.DeletePrefix("media/"); err != nil {
		t.Fatalf("DeletePrefix() error = %v", err)
	}
	if client.listVersionsCalls != 1 {
		t.Errorf("ListObjectVersions calls = %d, want 1 (then fall back)", client.listVersionsCalls)
	}
	if len(client.batchIdentifiers) != 2 {
		t.Fatalf("batch identifiers = %d, want 2", len(client.batchIdentifiers))
	}
	for _, id := range client.batchIdentifiers {
		if id.VersionId != nil {
			t.Errorf("VersionId for %s = %q, want nil on an unversioned delete", aws.ToString(id.Key), aws.ToString(id.VersionId))
		}
	}
}

func TestDeletePrefix_SkipsVersionListingForProvidersWithoutVersioning(t *testing.T) {
	client := &deleteCapturingClient{objects: twoObjects()}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderCloudflare)

	if err := s3m.DeletePrefix("media/"); err != nil {
		t.Fatalf("DeletePrefix() error = %v", err)
	}
	if client.listVersionsCalls != 0 {
		t.Errorf("ListObjectVersions calls = %d, want 0 for Cloudflare R2", client.listVersionsCalls)
	}
	if len(client.batchIdentifiers) != 2 {
		t.Errorf("batch identifiers = %d, want 2", len(client.batchIdentifiers))
	}
}

func TestDeleteObject_UnversionedFallbackMatchesExactKey(t *testing.T) {
	client := &deleteCapturingClient{objects: twoObjects()}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderDigitalOcean)

	if err := s3m.DeleteObject("/media/a.mov"); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
	if len(client.batchIdentifiers) != 1 || aws.ToString(client.batchIdentifiers[0].Key) != "media/a.mov" {
		t.Errorf("batch identifiers = %+v, want exactly media/a.mov", client.batchIdentifiers)
	}
}

func TestDeletePrefix_FallsBackToSingleDeletesWhenBatchUnsupported(t *testing.T) {
	client := &deleteCapturingClient{objects: twoObjects(), deleteObjectsErr: notImplemented}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderOther)
	client.listVersionsErr = &smithy.GenericAPIError{Code: "MethodNotAllowed"}

	if err := s3m.DeletePrefix("media/"); err != nil {
		t.Fatalf("DeletePrefix() error = %v", err)
	}
	if client.deleteObjectsCalled != 1 {
		t.Errorf("DeleteObjects calls = %d, want 1", client.deleteObjectsCalled)
	}
	if len(client.singleDeletedKeys) != 2 {
		t.Errorf("single deletes = %v, want both keys", client.singleDeletedKeys)
	}
}

func TestDeletePrefix_PropagatesOtherListingErrors(t *testing.T) {
	client := &deleteCapturingClient{listVersionsErr: &smithy.GenericAPIError{Code: "AccessDenied"}}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderMinIO)

	if err := s3m.DeletePrefix("media/"); err == nil {
		t.Fatal("DeletePrefix() expected an error for AccessDenied")
	}
	if client.deleteObjectsCalled != 0 {
		t.Errorf("DeleteObjects must not be called when listing fails")
	}
}

func TestDeletePrefix_AWSHandlesNilVersionFields(t *testing.T) {
	// MinIO-style responses for unversioned buckets omit VersionId/IsLatest/Size; AWS code paths must not panic.
	client := &deleteCapturingClient{versions: []s3types.ObjectVersion{
		{Key: aws.String("media/a.mov")},
		{Key: aws.String("media/b.mov"), VersionId: aws.String("v2"), IsLatest: aws.Bool(true), Size: aws.Int64(1)},
	}}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderAWS)

	if err := s3m.DeletePrefix("media/"); err != nil {
		t.Fatalf("DeletePrefix() error = %v", err)
	}
	if len(client.batchIdentifiers) != 2 {
		t.Fatalf("batch identifiers = %d, want 2", len(client.batchIdentifiers))
	}
	if client.batchIdentifiers[0].VersionId != nil {
		t.Errorf("first identifier should have no VersionId, got %q", aws.ToString(client.batchIdentifiers[0].VersionId))
	}
	if aws.ToString(client.batchIdentifiers[1].VersionId) != "v2" {
		t.Errorf("second identifier VersionId = %q, want v2", aws.ToString(client.batchIdentifiers[1].VersionId))
	}
}

func TestDeletePrefix_FallsBackToSingleDeletesWhenContentMD5Required(t *testing.T) {
	for _, code := range []string{"MissingContentMD5", "InvalidDigest"} {
		t.Run(code, func(t *testing.T) {
			client := &deleteCapturingClient{objects: twoObjects(), deleteObjectsErr: &smithy.GenericAPIError{Code: code}}
			s3m := newDeleteTestManager(client, configtypes.S3ProviderCloudflare)

			if err := s3m.DeletePrefix("media/"); err != nil {
				t.Fatalf("DeletePrefix() error = %v", err)
			}
			if len(client.singleDeletedKeys) != 2 {
				t.Errorf("single deletes = %v, want both keys", client.singleDeletedKeys)
			}
		})
	}
}

func TestDeletePrefix_DoesNotFallBackOnAccessDenied(t *testing.T) {
	client := &deleteCapturingClient{objects: twoObjects(), deleteObjectsErr: &smithy.GenericAPIError{Code: "AccessDenied"}}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderCloudflare)

	if err := s3m.DeletePrefix("media/"); err == nil {
		t.Fatal("DeletePrefix() expected an error for AccessDenied")
	}
	if len(client.singleDeletedKeys) != 0 {
		t.Errorf("single deletes = %v, want none", client.singleDeletedKeys)
	}
}

func TestDeleteObject_HandlesNilVersionFields(t *testing.T) {
	client := &deleteCapturingClient{versions: []s3types.ObjectVersion{{Key: aws.String("media/a.mov")}}}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderMinIO)

	if err := s3m.DeleteObject("media/a.mov"); err != nil {
		t.Fatalf("DeleteObject() error = %v", err)
	}
	if len(client.batchIdentifiers) != 1 || client.batchIdentifiers[0].VersionId != nil {
		t.Errorf("batch identifiers = %+v, want media/a.mov without VersionId", client.batchIdentifiers)
	}
}

func TestListObjects_ToleratesMissingKeyAndSize(t *testing.T) {
	client := &deleteCapturingClient{objects: []s3types.Object{{Key: aws.String("media/a.mov")}, {}}}
	s3m := newDeleteTestManager(client, configtypes.S3ProviderMinIO)

	output, err := s3m.ListObjects("media/")
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if len(output.S3Objects) != 2 || output.S3Objects[0].Size != 0 || output.S3Objects[1].Key != "" {
		t.Errorf("ListObjects() = %+v, want two entries with zero values for missing fields", output.S3Objects)
	}
}
