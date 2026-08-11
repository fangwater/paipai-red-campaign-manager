package lark

import (
	"context"
	"fmt"

	"paipai-red-campaign-manager/internal/coenzyme"
)

func (c *Client) FetchCoenzymeQ10Daily(ctx context.Context, wikiToken, preferredSheetID, preferredSheetName string) (coenzyme.Snapshot, error) {
	spreadsheetToken, err := c.resolveSpreadsheetToken(ctx, wikiToken)
	if err != nil {
		return coenzyme.Snapshot{}, fmt.Errorf("resolve coenzyme Q10 spreadsheet: %w", err)
	}
	sheetID, sheetName, rowCount, err := c.resolveSheet(ctx, spreadsheetToken, preferredSheetID, preferredSheetName)
	if err != nil {
		return coenzyme.Snapshot{}, fmt.Errorf("resolve coenzyme Q10 worksheet: %w", err)
	}
	values, err := c.readSheetRange(ctx, spreadsheetToken, fmt.Sprintf("%s!A1:S%d", sheetID, rowCount))
	if err != nil {
		return coenzyme.Snapshot{}, fmt.Errorf("read coenzyme Q10 daily values: %w", err)
	}
	records, err := coenzyme.ParseDailyValues(values)
	if err != nil {
		return coenzyme.Snapshot{}, fmt.Errorf("parse worksheet %q: %w", sheetName, err)
	}
	return coenzyme.Snapshot{
		WikiToken: wikiToken, SpreadsheetToken: spreadsheetToken,
		SheetID: sheetID, SheetName: sheetName, Records: records,
	}, nil
}
