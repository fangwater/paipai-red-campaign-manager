package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"paipai-red-campaign-manager/internal/config"

	larksdk "github.com/larksuite/oapi-sdk-go/v3"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	client := larksdk.NewClient(cfg.LarkAppID, cfg.LarkAppSecret)
	ctx := context.Background()
	found := false
	pageToken := ""
	for {
		builder := larkbitable.NewListAppTableReqBuilder().
			AppToken(cfg.LarkAppToken).
			PageSize(100)
		if pageToken != "" {
			builder.PageToken(pageToken)
		}

		resp, err := client.Bitable.AppTable.List(ctx, builder.Build())
		if err != nil {
			log.Fatal(err)
		}
		if !resp.Success() {
			log.Fatalf("list Bitable tables: code=%d message=%s", resp.Code, resp.Msg)
		}
		if resp.Data == nil {
			log.Fatal("Bitable returned an empty response")
		}

		for _, table := range resp.Data.Items {
			if table == nil || table.TableId == nil || table.Name == nil {
				continue
			}
			found = true
			fmt.Printf("%s\t%s\n", *table.TableId, *table.Name)
		}

		hasMore := resp.Data.HasMore != nil && *resp.Data.HasMore
		if !hasMore {
			break
		}
		if resp.Data.PageToken == nil || *resp.Data.PageToken == "" {
			log.Fatal("Bitable response has_more is true but page_token is empty")
		}
		pageToken = *resp.Data.PageToken
	}
	if !found {
		fmt.Fprintln(os.Stderr, "no tables found")
		os.Exit(1)
	}
}
