BEGIN;
CREATE TEMP TABLE probe_result (probe text PRIMARY KEY, observed text NOT NULL);
-- Synthetic writes stay inside this transaction. Store.dll leaves one audit row.
INSERT INTO public.event (identifier, timestamp, type, source, outcome, system, message, details)
SELECT identifier, timestamp, type, source, outcome, system, message, details
FROM public.event WHERE identifier = 'fixture-1';

DO $probe$
DECLARE
    ids integer;
    reported_constraint text;
BEGIN
    SELECT count(DISTINCT id) INTO ids FROM public.event WHERE identifier = 'fixture-1';
    IF ids <> 2 THEN RAISE EXCEPTION 'expected 2 physical keys, got %', ids; END IF;
    INSERT INTO probe_result VALUES ('event_duplicate_identifier_distinct_ids', ids::text);

    INSERT INTO public.documentationchunks (path, title, content)
        VALUES ('schema-probe.md', 'sample', 'first');
    BEGIN
        INSERT INTO public.documentationchunks (path, title, content)
            VALUES ('schema-probe.md', 'sample', 'second');
        RAISE EXCEPTION 'duplicate chunk was accepted';
    EXCEPTION WHEN unique_violation THEN
        GET STACKED DIAGNOSTICS reported_constraint = CONSTRAINT_NAME;
        IF reported_constraint IS DISTINCT FROM 'ix_doc_chunks_path_idx' THEN
            RAISE EXCEPTION 'unexpected unique index %', reported_constraint;
        END IF;
        INSERT INTO probe_result VALUES ('chunk_duplicate_default_index_sqlstate', SQLSTATE);
    END;
END
$probe$;

WITH relations AS (
    SELECT c.oid, c.relname
    FROM pg_catalog.pg_class AS c
    JOIN pg_catalog.pg_namespace AS n ON n.oid = c.relnamespace
    WHERE c.relkind = 'r' AND n.nspname = 'public'
        AND c.relname IN ('documentationchunks', 'event')
), tables AS (
    SELECT r.relname AS name,
        (SELECT jsonb_agg(jsonb_build_object(
            'name', a.attname, 'type', format_type(a.atttypid, a.atttypmod),
            'required', a.attnotnull, 'default', pg_get_expr(d.adbin, d.adrelid)
        ) ORDER BY a.attnum)
        FROM pg_attribute AS a LEFT JOIN pg_attrdef AS d
            ON (d.adrelid, d.adnum) = (a.attrelid, a.attnum)
        WHERE a.attrelid = r.oid AND a.attnum > 0 AND NOT a.attisdropped) AS columns,
        (SELECT coalesce(jsonb_agg(jsonb_build_object(
            'name', c.conname, 'definition', pg_get_constraintdef(c.oid),
            'validated', c.convalidated
        ) ORDER BY c.conname), '[]'::jsonb)
        FROM pg_constraint AS c WHERE c.conrelid = r.oid) AS constraints,
        (SELECT coalesce(jsonb_agg(jsonb_build_object(
            'name', i.relname, 'definition', pg_get_indexdef(i.oid),
            'valid', x.indisvalid, 'unique', x.indisunique, 'primary', x.indisprimary
        ) ORDER BY i.relname), '[]'::jsonb)
        FROM pg_index AS x JOIN pg_class AS i ON i.oid = x.indexrelid
        WHERE x.indrelid = r.oid) AS indexes
    FROM relations AS r
)
SELECT jsonb_pretty(jsonb_build_object(
    'postgres_version', current_setting('server_version'), 'schema', 'public',
    'probes', (SELECT jsonb_object_agg(probe, observed) FROM probe_result),
    'tables', jsonb_agg(to_jsonb(t) ORDER BY t.name)
))
FROM tables AS t;
ROLLBACK;
