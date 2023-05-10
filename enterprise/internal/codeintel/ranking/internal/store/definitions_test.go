package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/ranking/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestInsertDefinition(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(logger, t))
	store := New(&observation.TestContext, db)

	expectedDefinitions := []shared.RankingDefinitions{
		{
			UploadID:     1,
			SymbolName:   "foo",
			DocumentPath: "foo.go",
		},
		{
			UploadID:     1,
			SymbolName:   "bar",
			DocumentPath: "bar.go",
		},
		{
			UploadID:     1,
			SymbolName:   "foo",
			DocumentPath: "foo.go",
		},
	}

	// Insert definitions
	mockDefinitions := make(chan shared.RankingDefinitions, len(expectedDefinitions))
	for _, def := range expectedDefinitions {
		mockDefinitions <- def
	}
	close(mockDefinitions)
	if err := store.InsertDefinitionsForRanking(ctx, mockRankingGraphKey, mockDefinitions); err != nil {
		t.Fatalf("unexpected error inserting definitions: %s", err)
	}

	// Test definitions were inserted
	definitions, err := getRankingDefinitions(ctx, t, db, mockRankingGraphKey)
	if err != nil {
		t.Fatalf("unexpected error getting definitions: %s", err)
	}

	if diff := cmp.Diff(expectedDefinitions, definitions); diff != "" {
		t.Errorf("unexpected definitions (-want +got):\n%s", diff)
	}
}

//
//

func getRankingDefinitions(
	ctx context.Context,
	t *testing.T,
	db database.DB,
	graphKey string,
) (_ []shared.RankingDefinitions, err error) {
	query := fmt.Sprintf(
		`SELECT upload_id, symbol_name, document_path FROM codeintel_ranking_definitions WHERE graph_key = '%s'`,
		graphKey,
	)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var definitions []shared.RankingDefinitions
	for rows.Next() {
		var uploadID int
		var symbolName string
		var documentPath string
		err = rows.Scan(&uploadID, &symbolName, &documentPath)
		if err != nil {
			return nil, err
		}
		definitions = append(definitions, shared.RankingDefinitions{
			UploadID:     uploadID,
			SymbolName:   symbolName,
			DocumentPath: documentPath,
		})
	}

	return definitions, nil
}
