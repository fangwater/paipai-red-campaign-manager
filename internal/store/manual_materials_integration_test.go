package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestManualMaterialsIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := NewPostgres(ctx, databaseURL, "manual-materials-integration")
	if err != nil {
		t.Fatal(err)
	}
	defer postgres.Close()
	if err := postgres.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	materialID := hex.EncodeToString([]byte("manual-material-id-32bytes!!"))[:32]
	imageContent := []byte("manual-material-image")
	digest := sha256.Sum256(imageContent)
	assetID := hex.EncodeToString(digest[:])
	defer func() {
		cleanup := context.Background()
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM manual_materials WHERE material_id=$1", materialID)
		_, _ = postgres.pool.Exec(cleanup, "DELETE FROM manuscript_assets WHERE asset_id=$1", assetID)
	}()

	created, err := postgres.CreateManualMaterial(ctx, maituo.ManualMaterialInput{
		MaterialID: materialID,
		NoteID:     "6208dd8e000000002103e259",
		NoteURL:    "https://www.xiaohongshu.com/explore/6208dd8e000000002103e259",
		Title:      "早起精力记录",
		Body:       "连续记录两周睡眠和午后疲惫。",
		Comments:   []string{"想问剂量", "收藏了"},
		UploadedImages: []maituo.ManualMaterialImageInput{{
			AssetID: assetID, ContentType: "image/png", Width: 12, Height: 18, Content: imageContent,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.MaterialID != materialID || created.Title != "早起精力记录" ||
		created.NoteID != "6208dd8e000000002103e259" || created.Tagged ||
		created.ImageCount != 1 || created.CommentCount != 2 || created.Images[0].AssetID != assetID {
		t.Fatalf("created = %+v", created)
	}

	stored, found, err := postgres.ManualMaterial(ctx, materialID)
	if err != nil || !found || stored.Body != "连续记录两周睡眠和午后疲惫。" ||
		len(stored.Comments) != 2 || stored.Images[0].Width != 12 {
		t.Fatalf("stored = %+v found=%v err=%v", stored, found, err)
	}

	asset, assetFound, err := postgres.ManuscriptAsset(ctx, assetID)
	if err != nil || !assetFound || string(asset.Content) != string(imageContent) {
		t.Fatalf("asset=%+v found=%v err=%v", asset, assetFound, err)
	}

	listed, err := postgres.ManualMaterials(ctx, maituo.ManualMaterialsQuery{
		Search: "午后疲惫", Untagged: true, Page: 1, PageSize: 10,
	})
	if err != nil || listed.Total < 1 || listed.Items[0].MaterialID != materialID || listed.Items[0].Tagged {
		t.Fatalf("listed = %+v err=%v", listed, err)
	}

	tagged, found, err := postgres.UpdateManualMaterialTags(ctx, materialID, maituo.ManualMaterialTags{
		NoteType: "科普", CoverType: "大字报", CommercialIntensity: "软广",
		Audience: "职场人", UserScenario: "精力疲惫",
	})
	if err != nil || !found || !tagged.Tagged || tagged.Tags.Audience != "职场人" {
		t.Fatalf("tagged = %+v found=%v err=%v", tagged, found, err)
	}
	emptyUntagged, err := postgres.ManualMaterials(ctx, maituo.ManualMaterialsQuery{
		Untagged: true, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range emptyUntagged.Items {
		if item.MaterialID == materialID {
			t.Fatalf("tagged material still untagged = %+v", item)
		}
	}

	updated, found, err := postgres.UpdateManualMaterial(ctx, maituo.ManualMaterialInput{
		MaterialID:       materialID,
		NoteID:           "6208dd8e000000002103e259",
		NoteURL:          "https://www.xiaohongshu.com/explore/6208dd8e000000002103e259",
		Title:            "更新后的标题",
		Body:             "更新后的正文",
		Comments:         []string{"继续跟进"},
		ExistingImageIDs: []string{assetID},
	})
	if err != nil || !found || updated.Title != "更新后的标题" ||
		updated.CommentCount != 1 || updated.ImageCount != 1 {
		t.Fatalf("updated = %+v found=%v err=%v", updated, found, err)
	}
}
