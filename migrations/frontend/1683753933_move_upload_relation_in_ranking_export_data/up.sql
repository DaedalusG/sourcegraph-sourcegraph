TRUNCATE codeintel_ranking_definitions CASCADE;
TRUNCATE codeintel_ranking_references CASCADE;
TRUNCATE codeintel_initial_path_ranks CASCADE;

ALTER TABLE codeintel_ranking_definitions DROP COLUMN IF EXISTS upload_id;
ALTER TABLE codeintel_ranking_references DROP COLUMN IF EXISTS upload_id;
ALTER TABLE codeintel_initial_path_ranks DROP COLUMN IF EXISTS upload_id;

ALTER TABLE codeintel_ranking_definitions ADD COLUMN IF NOT EXISTS exported_upload_id INTEGER NOT NULL REFERENCES codeintel_ranking_exports(id);
ALTER TABLE codeintel_ranking_references ADD COLUMN IF NOT EXISTS exported_upload_id INTEGER NOT NULL REFERENCES codeintel_ranking_exports(id);
ALTER TABLE codeintel_initial_path_ranks ADD COLUMN IF NOT EXISTS exported_upload_id INTEGER NOT NULL REFERENCES codeintel_ranking_exports(id);

CREATE INDEX IF NOT EXISTS codeintel_ranking_definitions_exported_upload_id ON codeintel_ranking_definitions(exported_upload_id);
CREATE INDEX IF NOT EXISTS codeintel_ranking_references_exported_upload_id ON codeintel_ranking_references(exported_upload_id);
CREATE INDEX IF NOT EXISTS codeintel_initial_path_ranks_exported_upload_id ON codeintel_initial_path_ranks(exported_upload_id);
CREATE INDEX IF NOT EXISTS codeintel_ranking_definitions_graph_key_symbol_search ON codeintel_ranking_definitions(graph_key, symbol_name, exported_upload_id, document_path);

CREATE TABLE IF NOT EXISTS codeintel_ranking_definitions_janitor_queue (exported_upload_id INTEGER UNIQUE NOT NULL);
CREATE TABLE IF NOT EXISTS codeintel_ranking_references_janitor_queue (exported_upload_id INTEGER UNIQUE NOT NULL);
CREATE TABLE IF NOT EXISTS codeintel_ranking_paths_janitor_queue (exported_upload_id INTEGER UNIQUE NOT NULL);

CREATE OR REPLACE FUNCTION codeintel_ranking_janitor_enqueue() RETURNS trigger
    LANGUAGE plpgsql
    AS $$ BEGIN
    INSERT INTO codeintel_ranking_definitions_janitor_queue (exported_upload_id) SELECT id FROM oldtab;
    INSERT INTO codeintel_ranking_references_janitor_queue  (exported_upload_id) SELECT id FROM oldtab;
    INSERT INTO codeintel_ranking_paths_janitor_queue       (exported_upload_id) SELECT id FROM oldtab;
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS codeintel_ranking_exports_delete ON codeintel_ranking_exports;
CREATE TRIGGER codeintel_ranking_exports_delete AFTER DELETE ON codeintel_ranking_exports
REFERENCING OLD TABLE AS oldtab FOR EACH STATEMENT EXECUTE FUNCTION codeintel_ranking_janitor_enqueue();
