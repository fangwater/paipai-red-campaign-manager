package maituo

import (
	"os"
	"testing"
)

func TestUploadedSample(t *testing.T) {
	path := os.Getenv("MAITUO_SAMPLE_PATH")
	if path == "" {
		t.Skip("MAITUO_SAMPLE_PATH is not set")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	snapshot, err := Parse(file, "2026-07-23-MaiTuo-客户日报.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.KPIs) != 8 || len(snapshot.Notes) != 243 || len(snapshot.SPUs) != 2 || len(snapshot.Subaccounts) != 8 || len(snapshot.Trends) != 14 {
		t.Fatalf("unexpected row counts: kpis=%d notes=%d spus=%d subaccounts=%d trends=%d", len(snapshot.KPIs), len(snapshot.Notes), len(snapshot.SPUs), len(snapshot.Subaccounts), len(snapshot.Trends))
	}
	if snapshot.FileSHA256 != "34f7619f2db09d8f8233c99c87a5aee8c0e06208f2c7adb850f79d241d06082b" {
		t.Fatalf("unexpected SHA-256: %s", snapshot.FileSHA256)
	}
}

func TestParseXHSLinkUnitDelivery(t *testing.T) {
	raw := []byte(`{"target_template_id":226275,"keyword_gen_type":2,"keyword_target_period":7,"keyword_target_action":[1,2],"keyword_with_bids":[{"keyword_id":11,"keyword":"磷虾油","bid":125,"feed_bid":80,"keyword_source":14,"phrase_match_type":1}],"target_config":{"target_gender":"1","target_age":"28-32#33-100","target_city":"北京#上海","targetAreaCode":"1#2","target_device":"ios","target_device_price":"4000-5999","intelligent_expansion":1,"target_generalization_switch":1,"searchTargetCityIntent":"2","interest_keywords":["鱼油"],"keywords":["辅酶q10"],"reverse_target_crowd":["crowd-x"],"haveBrandInterestGroup":true,"have_category_interest_group":true,"crowd_target":{"crowd_pkg":[{"value":"2048_1","name":"心脑健康人群","groupId":"1","syncStatus":2}]},"industry_interest_target":{"content_interests":[{"code":"a","name":"医疗保健"}],"shopping_Interests":[{"name":"食品饮料","children":[{"name":"营养保健"}]}]},"premium_target_crowd":[{"id":"p1","name":"高价值人群","ratio":"1.5"}],"dandelion_crowd":{"normal_dandelion_crowd_list":[{"name":"蒲公英合作人群"}]}}}`)
	got := ParseXHSLinkUnitDelivery(raw)
	if got.TargetTemplateID != 226275 || got.KeywordGenType != 2 || got.SearchKeywordCount != 1 {
		t.Fatalf("unexpected delivery summary: %+v", got)
	}
	if got.SearchKeywords[0].Keyword != "磷虾油" || got.SearchKeywords[0].Bid != 125 {
		t.Fatalf("unexpected keyword: %+v", got.SearchKeywords[0])
	}
	if got.Target.Gender != "1" || got.Target.AreaCode != "1#2" || !got.Target.BrandInterestGroup || !got.Target.CategoryInterestGroup {
		t.Fatalf("unexpected basic target: %+v", got.Target)
	}
	if len(got.Target.CrowdPackages) != 1 || got.Target.CrowdPackages[0].Name != "心脑健康人群" || got.Target.CrowdPackages[0].SyncStatus != 2 {
		t.Fatalf("unexpected crowd packages: %+v", got.Target.CrowdPackages)
	}
	if len(got.Target.ContentInterests) != 1 || len(got.Target.ShoppingInterests) != 2 || len(got.Target.DandelionCrowds) != 1 {
		t.Fatalf("unexpected named targets: %+v", got.Target)
	}
}
