package store

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestInsertInitialPathRanks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	store := New(&observation.TestContext, db)

	mockUploadID := 1
	mockPathNames := make(chan string, 3)
	mockPathNames <- "foo.go"
	mockPathNames <- "bar.go"
	mockPathNames <- "baz.go"
	close(mockPathNames)
	if err := store.InsertInitialPathRanks(ctx, mockUploadID, mockUploadID, mockPathNames, 2, mockRankingGraphKey); err != nil {
		t.Fatalf("unexpected error inserting initial path counts: %s", err)
	}

	inputs, err := getInitialPathRanks(ctx, t, db, mockRankingGraphKey)
	if err != nil {
		t.Fatalf("unexpected error getting path count inputs: %s", err)
	}

	expectedInputs := []initialPathRanks{
		{UploadID: 1, DocumentPath: "bar.go"},
		{UploadID: 1, DocumentPath: "baz.go"},
		{UploadID: 1, DocumentPath: "foo.go"},
	}
	if diff := cmp.Diff(expectedInputs, inputs); diff != "" {
		t.Errorf("unexpected path count inputs (-want +got):\n%s", diff)
	}
}

//
//

type initialPathRanks struct {
	UploadID     int
	DocumentPath string
}

func getInitialPathRanks(
	ctx context.Context,
	t *testing.T,
	db database.DB,
	graphKey string,
) (pathRanks []initialPathRanks, err error) {
	query := sqlf.Sprintf(`
		SELECT upload_id, document_path FROM (
			SELECT
				upload_id,
				unnest(document_paths) AS document_path
			FROM codeintel_initial_path_ranks
			WHERE graph_key LIKE %s || '%%'
		)s
		GROUP BY upload_id, document_path
		ORDER BY upload_id, document_path
	`, graphKey)
	rows, err := db.QueryContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		var input initialPathRanks
		if err := rows.Scan(&input.UploadID, &input.DocumentPath); err != nil {
			return nil, err
		}

		pathRanks = append(pathRanks, input)
	}

	return pathRanks, nil
}
