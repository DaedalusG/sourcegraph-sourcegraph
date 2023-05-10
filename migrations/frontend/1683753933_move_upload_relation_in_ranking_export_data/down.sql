TRUNCATE codeintel_ranking_definitions CASCADE;
TRUNCATE codeintel_ranking_references CASCADE;
TRUNCATE codeintel_initial_path_ranks CASCADE;

DROP TRIGGER IF EXISTS codeintel_ranking_exports_delete ON codeintel_ranking_exports;
DROP FUNCTION IF EXISTS codeintel_ranking_janitor_enqueue;
DROP TABLE IF EXISTS codeintel_ranking_definitions_janitor_queue;
DROP TABLE IF EXISTS codeintel_ranking_references_janitor_queue;
DROP TABLE IF EXISTS codeintel_ranking_paths_janitor_queue;

ALTER TABLE codeintel_ranking_definitions DROP COLUMN IF EXISTS exported_upload_id;
ALTER TABLE codeintel_ranking_references DROP COLUMN IF EXISTS exported_upload_id;
ALTER TABLE codeintel_initial_path_ranks DROP COLUMN IF EXISTS exported_upload_id;

ALTER TABLE codeintel_ranking_definitions ADD COLUMN IF NOT EXISTS upload_id INTEGER NOT NULL;
ALTER TABLE codeintel_ranking_references ADD COLUMN IF NOT EXISTS upload_id INTEGER NOT NULL;
ALTER TABLE codeintel_initial_path_ranks ADD COLUMN IF NOT EXISTS upload_id INTEGER NOT NULL;

CREATE INDEX IF NOT EXISTS codeintel_ranking_definitions_graph_key_symbol_search ON codeintel_ranking_definitions(graph_key, symbol_name, upload_id, document_path);
CREATE INDEX IF NOT EXISTS codeintel_ranking_references_upload_id ON codeintel_ranking_references(upload_id);
CREATE INDEX IF NOT EXISTS codeintel_initial_path_ranks_upload_id ON codeintel_initial_path_ranks(upload_id);
