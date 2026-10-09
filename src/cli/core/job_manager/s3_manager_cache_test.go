package job_manager

import (
	"sync"
	"testing"

	transferapi "github.com/awslabs/filemoverexpress/core/transfer-api"
	"github.com/awslabs/filemoverexpress/types/configtypes"
)

func TestClearS3ManagerCache(t *testing.T) {
	tp := configtypes.TransferProfile{Name: "minio", Provider: configtypes.S3ProviderMinIO}
	cached := &transferapi.S3Manager{Lock: &sync.RWMutex{}}
	jm := &JobManager{
		s3ManagerLock:  &sync.RWMutex{},
		s3ManagerCache: map[string]*transferapi.S3Manager{tp.ConnectionKey(): cached},
	}

	got, err := jm.GetS3Manager(tp)
	if err != nil || got != cached {
		t.Fatalf("GetS3Manager() = (%p, %v), want the cached manager", got, err)
	}

	jm.ClearS3ManagerCache()
	if len(jm.s3ManagerCache) != 0 {
		t.Errorf("cache size after clear = %d, want 0", len(jm.s3ManagerCache))
	}
}
