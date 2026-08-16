package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (p *Postgres) CreateManualMaterial(ctx context.Context, input maituo.ManualMaterialInput) (maituo.ManualMaterial, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return maituo.ManualMaterial{}, fmt.Errorf("begin manual material create: %w", err)
	}
	defer tx.Rollback(ctx)

	comments := normalizeManualMaterialComments(input.Comments)
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO manual_materials (
			material_id, note_id, note_url, title, body, comments
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`, input.MaterialID, input.NoteID, input.NoteURL, input.Title, input.Body, comments).Scan(&createdAt, &updatedAt)
	if err != nil {
		return maituo.ManualMaterial{}, wrapManualMaterialWriteError("insert manual material", err)
	}
	images, err := attachManualMaterialImages(ctx, tx, input)
	if err != nil {
		return maituo.ManualMaterial{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return maituo.ManualMaterial{}, fmt.Errorf("commit manual material create: %w", err)
	}
	return composeManualMaterial(input, comments, images, createdAt, updatedAt, maituo.ManualMaterialTags{}), nil
}

func (p *Postgres) UpdateManualMaterial(ctx context.Context, input maituo.ManualMaterialInput) (maituo.ManualMaterial, bool, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return maituo.ManualMaterial{}, false, fmt.Errorf("begin manual material update: %w", err)
	}
	defer tx.Rollback(ctx)

	comments := normalizeManualMaterialComments(input.Comments)
	var createdAt, updatedAt time.Time
	var tags maituo.ManualMaterialTags
	err = tx.QueryRow(ctx, `
		UPDATE manual_materials
		SET note_id=$2, note_url=$3, title=$4, body=$5, comments=$6, updated_at=NOW()
		WHERE material_id=$1
		RETURNING created_at, updated_at, note_type, cover_type, commercial_intensity, audience, user_scenario
	`, input.MaterialID, input.NoteID, input.NoteURL, input.Title, input.Body, comments).Scan(
		&createdAt, &updatedAt,
		&tags.NoteType, &tags.CoverType, &tags.CommercialIntensity, &tags.Audience, &tags.UserScenario,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return maituo.ManualMaterial{}, false, nil
	}
	if err != nil {
		return maituo.ManualMaterial{}, false, wrapManualMaterialWriteError("update manual material", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM manual_material_assets WHERE material_id=$1`, input.MaterialID); err != nil {
		return maituo.ManualMaterial{}, false, fmt.Errorf("clear manual material images: %w", err)
	}
	images, err := attachManualMaterialImages(ctx, tx, input)
	if err != nil {
		return maituo.ManualMaterial{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return maituo.ManualMaterial{}, false, fmt.Errorf("commit manual material update: %w", err)
	}
	return composeManualMaterial(input, comments, images, createdAt, updatedAt, tags), true, nil
}

func (p *Postgres) UpdateManualMaterialTags(ctx context.Context, materialID string, tags maituo.ManualMaterialTags) (maituo.ManualMaterial, bool, error) {
	var createdAt, updatedAt time.Time
	item := maituo.ManualMaterial{
		MaterialID: materialID,
		Comments:   []string{},
		Images:     []maituo.ManualMaterialImage{},
	}
	err := p.pool.QueryRow(ctx, `
		UPDATE manual_materials
		SET note_type=$2, cover_type=$3, commercial_intensity=$4, audience=$5, user_scenario=$6, updated_at=NOW()
		WHERE material_id=$1
		RETURNING note_id, note_url, title, body, comments, note_type, cover_type,
			commercial_intensity, audience, user_scenario, created_at, updated_at
	`, materialID, tags.NoteType, tags.CoverType, tags.CommercialIntensity, tags.Audience, tags.UserScenario).Scan(
		&item.NoteID, &item.NoteURL, &item.Title, &item.Body, &item.Comments,
		&item.Tags.NoteType, &item.Tags.CoverType, &item.Tags.CommercialIntensity,
		&item.Tags.Audience, &item.Tags.UserScenario, &createdAt, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return maituo.ManualMaterial{}, false, nil
	}
	if err != nil {
		return maituo.ManualMaterial{}, false, fmt.Errorf("update manual material tags: %w", err)
	}
	images, err := p.listManualMaterialImages(ctx, materialID)
	if err != nil {
		return maituo.ManualMaterial{}, false, err
	}
	if item.Comments == nil {
		item.Comments = []string{}
	}
	item.Images = images
	item.ImageCount = len(images)
	item.CommentCount = len(item.Comments)
	item.Tagged = item.Tags.Complete()
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, true, nil
}

func (p *Postgres) ManualMaterial(ctx context.Context, materialID string) (maituo.ManualMaterial, bool, error) {
	item := maituo.ManualMaterial{MaterialID: materialID, Comments: []string{}, Images: []maituo.ManualMaterialImage{}}
	err := p.pool.QueryRow(ctx, `
		SELECT note_id, note_url, title, body, comments, note_type, cover_type,
			commercial_intensity, audience, user_scenario, created_at, updated_at
		FROM manual_materials
		WHERE material_id=$1
	`, materialID).Scan(
		&item.NoteID, &item.NoteURL, &item.Title, &item.Body, &item.Comments,
		&item.Tags.NoteType, &item.Tags.CoverType, &item.Tags.CommercialIntensity,
		&item.Tags.Audience, &item.Tags.UserScenario, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return maituo.ManualMaterial{}, false, nil
	}
	if err != nil {
		return maituo.ManualMaterial{}, false, fmt.Errorf("query manual material: %w", err)
	}
	images, err := p.listManualMaterialImages(ctx, materialID)
	if err != nil {
		return maituo.ManualMaterial{}, false, err
	}
	if item.Comments == nil {
		item.Comments = []string{}
	}
	item.Images = images
	item.ImageCount = len(images)
	item.CommentCount = len(item.Comments)
	item.Tagged = item.Tags.Complete()
	return item, true, nil
}

func (p *Postgres) ManualMaterials(ctx context.Context, query maituo.ManualMaterialsQuery) (maituo.ManualMaterials, error) {
	result := maituo.ManualMaterials{
		Search: query.Search, Untagged: query.Untagged, Page: query.Page, PageSize: query.PageSize,
		Items: []maituo.ManualMaterial{},
	}
	searchPattern := "%" + strings.TrimSpace(query.Search) + "%"
	if err := p.pool.QueryRow(ctx, `
		SELECT
			ARRAY(SELECT DISTINCT NULLIF(BTRIM(note_type), '') FROM manual_materials WHERE BTRIM(note_type) <> '' ORDER BY 1),
			ARRAY(SELECT DISTINCT NULLIF(BTRIM(cover_type), '') FROM manual_materials WHERE BTRIM(cover_type) <> '' ORDER BY 1),
			ARRAY(SELECT DISTINCT NULLIF(BTRIM(commercial_intensity), '') FROM manual_materials WHERE BTRIM(commercial_intensity) <> '' ORDER BY 1),
			ARRAY(SELECT DISTINCT NULLIF(BTRIM(audience), '') FROM manual_materials WHERE BTRIM(audience) <> '' ORDER BY 1),
			ARRAY(SELECT DISTINCT NULLIF(BTRIM(user_scenario), '') FROM manual_materials WHERE BTRIM(user_scenario) <> '' ORDER BY 1)
	`).Scan(
		&result.TagOptions.NoteType, &result.TagOptions.CoverType, &result.TagOptions.CommercialIntensity,
		&result.TagOptions.Audience, &result.TagOptions.UserScenario,
	); err != nil {
		return result, fmt.Errorf("query manual material tag options: %w", err)
	}
	if err := p.pool.QueryRow(ctx, `
		SELECT COUNT(*)::INTEGER
		FROM manual_materials
		WHERE ($1 = '%%'
		   OR title ILIKE $1
		   OR body ILIKE $1
		   OR note_id ILIKE $1
		   OR note_url ILIKE $1
		   OR EXISTS (
			SELECT 1 FROM unnest(comments) AS comment(value)
			WHERE comment.value ILIKE $1
		   ))
		  AND (NOT $2 OR note_type = '' OR cover_type = '' OR commercial_intensity = '' OR audience = '' OR user_scenario = '')
	`, searchPattern, query.Untagged).Scan(&result.Total); err != nil {
		return result, fmt.Errorf("query manual material count: %w", err)
	}
	if result.Total == 0 {
		return result, nil
	}
	offset := (query.Page - 1) * query.PageSize
	rows, err := p.pool.Query(ctx, `
		SELECT material.material_id, material.note_id, material.note_url, material.title, material.body, material.comments,
			material.note_type, material.cover_type, material.commercial_intensity, material.audience, material.user_scenario,
			material.created_at, material.updated_at,
			COALESCE((
				SELECT ARRAY_AGG(assets.asset_id ORDER BY assets.position)
				FROM manual_material_assets assets
				WHERE assets.material_id=material.material_id
			), '{}'::TEXT[]),
			COALESCE((
				SELECT ARRAY_AGG(assets.width ORDER BY assets.position)
				FROM manual_material_assets assets
				WHERE assets.material_id=material.material_id
			), '{}'::INTEGER[]),
			COALESCE((
				SELECT ARRAY_AGG(assets.height ORDER BY assets.position)
				FROM manual_material_assets assets
				WHERE assets.material_id=material.material_id
			), '{}'::INTEGER[])
		FROM manual_materials material
		WHERE ($1 = '%%'
		   OR material.title ILIKE $1
		   OR material.body ILIKE $1
		   OR material.note_id ILIKE $1
		   OR material.note_url ILIKE $1
		   OR EXISTS (
			SELECT 1 FROM unnest(material.comments) AS comment(value)
			WHERE comment.value ILIKE $1
		   ))
		  AND (NOT $2 OR material.note_type = '' OR material.cover_type = '' OR material.commercial_intensity = '' OR material.audience = '' OR material.user_scenario = '')
		ORDER BY material.updated_at DESC, material.material_id
		LIMIT $3 OFFSET $4
	`, searchPattern, query.Untagged, query.PageSize, offset)
	if err != nil {
		return result, fmt.Errorf("query manual materials: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item maituo.ManualMaterial
		var assetIDs []string
		var widths, heights []int
		if err := rows.Scan(
			&item.MaterialID, &item.NoteID, &item.NoteURL, &item.Title, &item.Body, &item.Comments,
			&item.Tags.NoteType, &item.Tags.CoverType, &item.Tags.CommercialIntensity,
			&item.Tags.Audience, &item.Tags.UserScenario,
			&item.CreatedAt, &item.UpdatedAt, &assetIDs, &widths, &heights,
		); err != nil {
			return result, fmt.Errorf("scan manual material: %w", err)
		}
		if item.Comments == nil {
			item.Comments = []string{}
		}
		item.Images = make([]maituo.ManualMaterialImage, 0, len(assetIDs))
		for index, assetID := range assetIDs {
			item.Images = append(item.Images, maituo.ManualMaterialImage{
				AssetID: assetID, Width: widths[index], Height: heights[index],
			})
		}
		item.ImageCount = len(item.Images)
		item.CommentCount = len(item.Comments)
		item.Tagged = item.Tags.Complete()
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterate manual materials: %w", err)
	}
	return result, nil
}

func attachManualMaterialImages(ctx context.Context, tx pgx.Tx, input maituo.ManualMaterialInput) ([]maituo.ManualMaterialImage, error) {
	for _, image := range input.UploadedImages {
		if _, err := tx.Exec(ctx, `
			INSERT INTO manuscript_assets (
				asset_id, content_type, byte_size, width, height, content
			) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (asset_id) DO NOTHING
		`, image.AssetID, image.ContentType, len(image.Content), image.Width, image.Height, image.Content); err != nil {
			return nil, fmt.Errorf("insert manual material image: %w", err)
		}
	}

	images := make([]maituo.ManualMaterialImage, 0, len(input.ExistingImageIDs)+len(input.UploadedImages))
	position := 0
	for _, assetID := range input.ExistingImageIDs {
		var width, height int
		err := tx.QueryRow(ctx, `
			SELECT width, height
			FROM manuscript_assets
			WHERE asset_id=$1
		`, assetID).Scan(&width, &height)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("existing image %s was not found", assetID)
		}
		if err != nil {
			return nil, fmt.Errorf("lookup existing image %s: %w", assetID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO manual_material_assets (material_id, position, asset_id, width, height)
			VALUES ($1, $2, $3, $4, $5)
		`, input.MaterialID, position, assetID, width, height); err != nil {
			return nil, fmt.Errorf("link existing image %s: %w", assetID, err)
		}
		images = append(images, maituo.ManualMaterialImage{AssetID: assetID, Width: width, Height: height})
		position++
	}
	for _, image := range input.UploadedImages {
		if _, err := tx.Exec(ctx, `
			INSERT INTO manual_material_assets (material_id, position, asset_id, width, height)
			VALUES ($1, $2, $3, $4, $5)
		`, input.MaterialID, position, image.AssetID, image.Width, image.Height); err != nil {
			return nil, fmt.Errorf("link uploaded image %s: %w", image.AssetID, err)
		}
		images = append(images, maituo.ManualMaterialImage{
			AssetID: image.AssetID, Width: image.Width, Height: image.Height,
		})
		position++
	}
	return images, nil
}

func (p *Postgres) listManualMaterialImages(ctx context.Context, materialID string) ([]maituo.ManualMaterialImage, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT asset_id, width, height
		FROM manual_material_assets
		WHERE material_id=$1
		ORDER BY position
	`, materialID)
	if err != nil {
		return nil, fmt.Errorf("query manual material images: %w", err)
	}
	defer rows.Close()
	images := []maituo.ManualMaterialImage{}
	for rows.Next() {
		var image maituo.ManualMaterialImage
		if err := rows.Scan(&image.AssetID, &image.Width, &image.Height); err != nil {
			return nil, fmt.Errorf("scan manual material image: %w", err)
		}
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate manual material images: %w", err)
	}
	return images, nil
}

func composeManualMaterial(
	input maituo.ManualMaterialInput,
	comments []string,
	images []maituo.ManualMaterialImage,
	createdAt, updatedAt time.Time,
	tags maituo.ManualMaterialTags,
) maituo.ManualMaterial {
	return maituo.ManualMaterial{
		MaterialID:   input.MaterialID,
		NoteID:       input.NoteID,
		NoteURL:      input.NoteURL,
		Title:        input.Title,
		Body:         input.Body,
		Comments:     comments,
		Tags:         tags,
		Tagged:       tags.Complete(),
		Images:       images,
		ImageCount:   len(images),
		CommentCount: len(comments),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func normalizeManualMaterialComments(comments []string) []string {
	if comments == nil {
		return []string{}
	}
	return comments
}

func wrapManualMaterialWriteError(action string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "idx_manual_materials_note_id" {
		return fmt.Errorf("该笔记 ID 已存在")
	}
	return fmt.Errorf("%s: %w", action, err)
}
