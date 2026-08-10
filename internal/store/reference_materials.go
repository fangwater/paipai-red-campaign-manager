package store

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
)

const referenceMaterialsCTE = `
	WITH relationships AS (
		SELECT LOWER(BTRIM(reference.reference_note_id)) AS reference_note_id,
			notes.note_id AS source_note_id
		FROM service_provider_notes notes
		CROSS JOIN LATERAL unnest(
			COALESCE(notes.reference_note_ids, '{}'::TEXT[])
		) AS reference(reference_note_id)
		WHERE BTRIM(reference.reference_note_id) ~* '^[0-9a-f]{24}$'
		  AND LOWER(BTRIM(reference.reference_note_id)) <> LOWER(BTRIM(notes.note_id))
		GROUP BY LOWER(BTRIM(reference.reference_note_id)), notes.note_id
	), source_providers AS (
		SELECT executions.note_id AS source_note_id,
			COALESCE(
				ARRAY_AGG(DISTINCT tables.provider_name ORDER BY tables.provider_name)
					FILTER (WHERE tables.provider_name IS NOT NULL),
				'{}'::TEXT[]
			) AS providers
		FROM service_provider_note_executions executions
		JOIN service_provider_content_tables tables
		  ON tables.provider_code=executions.provider_code
		WHERE executions.deleted_at IS NULL
		GROUP BY executions.note_id
	), filtered_relationships AS (
		SELECT relationships.reference_note_id, relationships.source_note_id,
			COALESCE(source_providers.providers, '{}'::TEXT[]) AS providers
		FROM relationships
		LEFT JOIN source_providers USING (source_note_id)
		WHERE $1 = '%%'
		   OR relationships.reference_note_id ILIKE $1
		   OR relationships.source_note_id ILIKE $1
		   OR EXISTS (
			SELECT 1
			FROM unnest(COALESCE(source_providers.providers, '{}'::TEXT[])) AS provider(name)
			WHERE provider.name ILIKE $1
		   )
	), content_status AS (
		SELECT materials.reference_note_id,
			EXISTS (
				SELECT 1 FROM reference_material_contents content
				WHERE content.reference_note_id=materials.reference_note_id
				  AND BTRIM(content.note_content) <> ''
			) AS has_manual_content,
			EXISTS (
				SELECT 1 FROM service_provider_notes referenced
				WHERE LOWER(BTRIM(referenced.note_id))=materials.reference_note_id
				  AND BTRIM(referenced.note_content) <> ''
			) AS has_synced_content
		FROM (
			SELECT DISTINCT reference_note_id FROM filtered_relationships
		) materials
	)
`

func (p *Postgres) MaituoReferenceMaterials(ctx context.Context, query maituo.ReferenceMaterialsQuery) (maituo.ReferenceMaterials, error) {
	result := maituo.ReferenceMaterials{
		Search: query.Search, Page: query.Page, PageSize: query.PageSize,
		Items: []maituo.ReferenceMaterialItem{},
	}
	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	statsQuery := referenceMaterialsCTE + `
		SELECT COUNT(DISTINCT reference_note_id)::INTEGER,
			COUNT(DISTINCT source_note_id)::INTEGER,
			COUNT(*)::INTEGER,
			(
				SELECT COUNT(DISTINCT provider.name)::INTEGER
				FROM filtered_relationships material
				CROSS JOIN LATERAL unnest(material.providers) AS provider(name)
			)
		FROM filtered_relationships
	`
	if err := p.pool.QueryRow(ctx, statsQuery, searchPattern).Scan(
		&result.Stats.MaterialCount,
		&result.Stats.SourceNoteCount,
		&result.Stats.ReferenceCount,
		&result.Stats.ProviderCount,
	); err != nil {
		return result, fmt.Errorf("query reference material stats: %w", err)
	}
	result.Total = result.Stats.MaterialCount
	if result.Total == 0 {
		return result, nil
	}

	offset := (query.Page - 1) * query.PageSize
	rowsQuery := referenceMaterialsCTE + `
		SELECT material.reference_note_id,
			ARRAY_AGG(DISTINCT material.source_note_id ORDER BY material.source_note_id),
			COALESCE((
				SELECT ARRAY_AGG(DISTINCT provider.name ORDER BY provider.name)
				FROM filtered_relationships provider_material
				CROSS JOIN LATERAL unnest(provider_material.providers) AS provider(name)
				WHERE provider_material.reference_note_id=material.reference_note_id
			), '{}'::TEXT[]),
			COUNT(DISTINCT material.source_note_id)::INTEGER AS usage_count,
			(status.has_manual_content OR status.has_synced_content) AS has_content,
			CASE
				WHEN status.has_manual_content THEN 'manual'
				WHEN status.has_synced_content THEN 'manuscript'
				ELSE ''
			END AS content_source
		FROM filtered_relationships material
		JOIN content_status status USING (reference_note_id)
		GROUP BY material.reference_note_id, status.has_manual_content, status.has_synced_content
		ORDER BY usage_count DESC, material.reference_note_id
		LIMIT $2 OFFSET $3
	`
	rows, err := p.pool.Query(ctx, rowsQuery, searchPattern, query.PageSize, offset)
	if err != nil {
		return result, fmt.Errorf("query reference materials: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item maituo.ReferenceMaterialItem
		if err := rows.Scan(
			&item.ReferenceNoteID, &item.SourceNoteIDs, &item.Providers, &item.UsageCount,
			&item.HasContent, &item.ContentSource,
		); err != nil {
			return result, fmt.Errorf("scan reference material: %w", err)
		}
		item.NoteURL = referenceMaterialURL(item.ReferenceNoteID)
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate reference materials: %w", err)
	}
	return result, nil
}

func (p *Postgres) MaituoReferenceMaterialContent(ctx context.Context, noteID string) (maituo.ReferenceMaterialContent, error) {
	result := maituo.ReferenceMaterialContent{
		ReferenceNoteID: noteID,
		NoteURL:         referenceMaterialURL(noteID),
	}
	var updatedAt time.Time
	err := p.pool.QueryRow(ctx, `
		WITH material AS (
			SELECT 1
			FROM service_provider_notes notes
			CROSS JOIN LATERAL unnest(
				COALESCE(notes.reference_note_ids, '{}'::TEXT[])
			) AS reference(reference_note_id)
			WHERE LOWER(BTRIM(reference.reference_note_id))=$1
			LIMIT 1
		), candidates AS (
			SELECT content.note_content, 'manual'::TEXT AS source,
				content.updated_at, 1 AS priority
			FROM reference_material_contents content
			JOIN material ON TRUE
			WHERE content.reference_note_id=$1
			  AND BTRIM(content.note_content) <> ''
			UNION ALL
			SELECT notes.note_content, 'manuscript'::TEXT AS source,
				notes.updated_at, 2 AS priority
			FROM service_provider_notes notes
			JOIN material ON TRUE
			WHERE LOWER(BTRIM(notes.note_id))=$1
			  AND BTRIM(notes.note_content) <> ''
		)
		SELECT note_content, source, updated_at
		FROM candidates
		ORDER BY priority
		LIMIT 1
	`, noteID).Scan(&result.NoteContent, &result.Source, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("query reference material content: %w", err)
	}
	result.Found = true
	result.UpdatedAt = &updatedAt
	return result, nil
}

func (p *Postgres) SaveMaituoReferenceMaterialContent(
	ctx context.Context,
	input maituo.ReferenceMaterialContentInput,
) (maituo.ReferenceMaterialContent, bool, error) {
	result := maituo.ReferenceMaterialContent{
		ReferenceNoteID: input.ReferenceNoteID,
		NoteURL:         referenceMaterialURL(input.ReferenceNoteID),
		Found:           true,
		Source:          "manual",
	}
	var updatedAt time.Time
	err := p.pool.QueryRow(ctx, `
		WITH material AS (
			SELECT 1
			FROM service_provider_notes notes
			CROSS JOIN LATERAL unnest(
				COALESCE(notes.reference_note_ids, '{}'::TEXT[])
			) AS reference(reference_note_id)
			WHERE LOWER(BTRIM(reference.reference_note_id))=$1
			LIMIT 1
		)
		INSERT INTO reference_material_contents (
			reference_note_id, note_content
		)
		SELECT $1, $2
		FROM material
		ON CONFLICT (reference_note_id) DO UPDATE SET
			note_content=EXCLUDED.note_content,
			updated_at=NOW()
		RETURNING note_content, updated_at
	`, input.ReferenceNoteID, input.NoteContent).Scan(&result.NoteContent, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return maituo.ReferenceMaterialContent{}, false, nil
	}
	if err != nil {
		return maituo.ReferenceMaterialContent{}, false, fmt.Errorf("save reference material content: %w", err)
	}
	result.UpdatedAt = &updatedAt
	return result, true, nil
}

func referenceMaterialURL(noteID string) string {
	return "https://www.xiaohongshu.com/explore/" + url.PathEscape(noteID)
}
