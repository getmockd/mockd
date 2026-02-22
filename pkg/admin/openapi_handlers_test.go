package admin

import (
	"context"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

func TestGetAllMocksForExport_FallsBackToDataStoreWhenEngineUnavailable(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()), WithLocalEngineClient(engineclient.New("http://localhost:99999")))
	defer api.Stop()

	mocks, err := api.getAllMocksForExport(context.Background())
	if err != nil {
		t.Fatalf("expected fallback to datastore, got error: %v", err)
	}
	if mocks == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(mocks) != 0 {
		t.Fatalf("expected 0 mocks from empty datastore, got %d", len(mocks))
	}
}
