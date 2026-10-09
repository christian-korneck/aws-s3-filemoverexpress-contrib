package transfer_api

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/awslabs/filemoverexpress/constants"
	"github.com/awslabs/filemoverexpress/core/transfer-api/mock"
	"github.com/awslabs/filemoverexpress/types/configtypes"
)

func TestS3Manager_Upload(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "TestS3Manager_Upload")
	err := os.MkdirAll(tmpDir, os.ModePerm)
	if err != nil {
		t.Errorf("TestS3Manager_Upload failed creating test dir: %s", err)
	}
	testFilePath := filepath.Join(tmpDir, "file.txt")
	file, err := os.Create(testFilePath)
	if err != nil {
		t.Errorf("TestS3Manager_Upload failed creating test file: %s", err)
	}
	type fields struct {
		AwsProfile string
		Bucket     string
		Client     FileMoverS3ClientInterface
		Region     string
		Uploader   *manager.Uploader
	}
	type args struct {
		input UploadConfig
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Upload API with no Uploader",
			fields: fields{
				AwsProfile: mock.UniteTestMockAWSProfile,
				Bucket:     mock.UnitTestMockBucket,
				Region:     mock.UnitTestMockRegion,
				Uploader:   nil,
			},
			args: args{
				input: UploadConfig{
					AutoTune:          false,
					Checksum:          "5fa8d142d3a24dd9448385db95c374ae",
					ChecksumAlgorithm: "md5",
					ChunkSize:         5 * constants.MiB,
					Context:           context.Background(),
					FilePath:          testFilePath,
					Reader: &FileReader{
						File:  file,
						Size:  0,
						Start: time.Now(),
					},
					Destination:  "my/s3/path/to/file.txt",
					StorageClass: "STANDARD",
					Threads:      1,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3m := &S3Manager{
				AwsProfile: tt.fields.AwsProfile,
				Bucket:     tt.fields.Bucket,
				Client:     &mock.FileMoverS3Client{},
				Region:     tt.fields.Region,
				Uploader:   tt.fields.Uploader,
				Lock:       &sync.RWMutex{},
			}
			if _, err := s3m.Upload(tt.args.input); (err != nil) != tt.wantErr {
				t.Errorf("Upload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
	file.Close()
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Errorf("Failed to remove test data files %s: %s", tmpDir, err.Error())
	}
}

func Test_convertToAWSStorageClass(t *testing.T) {
	type args struct {
		storageClass string
	}
	tests := []struct {
		name    string
		args    args
		want    types.StorageClass
		wantErr bool
	}{
		{
			name: "Valid storage class",
			args: args{
				storageClass: "Intelligent Tiering",
			},
			want:    types.StorageClassIntelligentTiering,
			wantErr: false,
		},
		{
			name: "Invalid storage class",
			args: args{
				storageClass: "Invalid",
			},
			want:    types.StorageClassStandard,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToAWSStorageClass(tt.args.storageClass)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToAWSStorageClass() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("convertToAWSStorageClass() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// multipartCapturingClient records the inputs of a multipart upload. The embedded interface is nil, so any
// call the uploader is not expected to make panics.
type multipartCapturingClient struct {
	FileMoverS3ClientInterface

	lock         sync.Mutex
	create       *s3.CreateMultipartUploadInput
	parts        []*s3.UploadPartInput
	complete     *s3.CompleteMultipartUploadInput
	putObjectsIn []*s3.PutObjectInput
}

func (c *multipartCapturingClient) PutObject(_ context.Context, params *s3.PutObjectInput,
	_ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.putObjectsIn = append(c.putObjectsIn, params)
	return &s3.PutObjectOutput{}, nil
}

func (c *multipartCapturingClient) CreateMultipartUpload(_ context.Context, params *s3.CreateMultipartUploadInput,
	_ ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	c.create = params
	return &s3.CreateMultipartUploadOutput{UploadId: aws.String("upload-1")}, nil
}

func (c *multipartCapturingClient) UploadPart(_ context.Context, params *s3.UploadPartInput,
	_ ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.parts = append(c.parts, params)
	return &s3.UploadPartOutput{ETag: aws.String("etag")}, nil
}

func (c *multipartCapturingClient) CompleteMultipartUpload(_ context.Context, params *s3.CompleteMultipartUploadInput,
	_ ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	c.complete = params
	return &s3.CompleteMultipartUploadOutput{}, nil
}

func (c *multipartCapturingClient) AbortMultipartUpload(_ context.Context, _ *s3.AbortMultipartUploadInput,
	_ ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	return &s3.AbortMultipartUploadOutput{}, nil
}

func uploadTwoParts(t *testing.T, provider configtypes.S3Provider, storageClass string) *multipartCapturingClient {
	t.Helper()
	client := &multipartCapturingClient{}
	s3m := &S3Manager{Bucket: "bucket", Client: client, Lock: &sync.RWMutex{}, Provider: provider}
	_, err := s3m.Upload(UploadConfig{
		ChunkSize:    5 * constants.MiB,
		Context:      context.Background(),
		Destination:  "media/clip.mov",
		Reader:       bytes.NewReader(make([]byte, 6*constants.MiB)),
		StorageClass: storageClass,
		Threads:      1,
	})
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if client.create == nil || len(client.parts) != 2 {
		t.Fatalf("expected a two-part multipart upload, got create=%v parts=%d", client.create != nil, len(client.parts))
	}
	return client
}

func TestUpload_MultipartChecksumsPerProvider(t *testing.T) {
	minio := uploadTwoParts(t, configtypes.S3ProviderMinIO, "standard")
	if minio.create.ChecksumAlgorithm != "" {
		t.Errorf("MinIO CreateMultipartUpload ChecksumAlgorithm = %q, want none", minio.create.ChecksumAlgorithm)
	}
	for i, part := range minio.parts {
		if part.ChecksumAlgorithm != "" {
			t.Errorf("MinIO part %d ChecksumAlgorithm = %q, want none", i, part.ChecksumAlgorithm)
		}
	}

	aws := uploadTwoParts(t, configtypes.S3ProviderAWS, "standard")
	if aws.create.ChecksumAlgorithm != types.ChecksumAlgorithmCrc32 {
		t.Errorf("AWS CreateMultipartUpload ChecksumAlgorithm = %q, want SDK default CRC32", aws.create.ChecksumAlgorithm)
	}
}

func TestUpload_StorageClassPerProvider(t *testing.T) {
	tests := []struct {
		provider     configtypes.S3Provider
		storageClass string
		want         types.StorageClass
	}{
		{provider: configtypes.S3ProviderAWS, storageClass: "glacier_ir", want: types.StorageClassGlacierIr},
		{provider: configtypes.S3ProviderBackblaze, storageClass: "glacier_ir", want: ""},
		{provider: configtypes.S3ProviderWasabi, storageClass: "standard", want: ""},
		{provider: configtypes.S3ProviderMinIO, storageClass: "reduced redundancy", want: types.StorageClass("REDUCED_REDUNDANCY")},
	}
	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			client := uploadTwoParts(t, tt.provider, tt.storageClass)
			if client.create.StorageClass != tt.want {
				t.Errorf("StorageClass = %q, want %q", client.create.StorageClass, tt.want)
			}
		})
	}
}
