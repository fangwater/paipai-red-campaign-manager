package store

import (
	"context"
	"fmt"

	"paipai-red-campaign-manager/internal/maituo"

	"github.com/jackc/pgx/v5"
)

func (p *Postgres) ManuscriptAsset(ctx context.Context, assetID string) (maituo.ManuscriptAsset, bool, error) {
	var asset maituo.ManuscriptAsset
	err := p.pool.QueryRow(ctx, `
		SELECT assets.asset_id, assets.content_type, assets.content, assets.created_at
		FROM manuscript_assets assets
		WHERE assets.asset_id = $1
		  AND (
			EXISTS (
				SELECT 1 FROM service_provider_note_assets links
				WHERE links.asset_id = assets.asset_id
			)
			OR EXISTS (
				SELECT 1 FROM manual_material_assets links
				WHERE links.asset_id = assets.asset_id
			)
		  )
	`, assetID).Scan(&asset.AssetID, &asset.ContentType, &asset.Content, &asset.CreatedAt)
	if err == nil {
		return asset, true, nil
	}
	if err == pgx.ErrNoRows {
		return maituo.ManuscriptAsset{}, false, nil
	}
	return maituo.ManuscriptAsset{}, false, fmt.Errorf("query manuscript asset: %w", err)
}
