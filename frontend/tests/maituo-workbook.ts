import ExcelJS from "exceljs";

export async function makeMaituoWorkbook(marker: string, includedSheets?: string[]): Promise<Buffer> {
  const workbook = new ExcelJS.Workbook();
  const sheets: Array<[string, string[]]> = [
    ["总览KPI", ["指标", "数值", "数据口径"]],
    ["笔记明细", ["笔记ID", "笔记链接", "分类", "子账户", "计划名", "场域", "词类备注", "消耗", "回搜人数", "回搜成本", "预计回流后成本", "回搜率(%)", "CPC", "CTR(%)"]],
    ["分SPU总览", ["SPU", "竞价消耗", "回搜", "回搜成本", "回搜率(%)", "CPC", "CTR(%)", "笔记数"]],
    ["分子账户", ["SPU", "子账户", "场域", "回搜成本", "预计回流后成本", "消耗", "回搜", "回搜率(%)", "CPC", "CTR(%)", "笔记数"]],
    ["淘搜趋势", ["日期", "辅酶消耗(元)", "辅酶淘搜UV", "辅酶成交UV", "辅酶淘搜成本(元/人)", "磷虾油消耗(元)", "磷虾油淘搜UV", "磷虾油成交UV", "磷虾油淘搜成本(元/人)", "合计淘搜UV", "合计成交UV", "合计淘搜成本(元/人)", "合计消耗(元)", "合计回搜成本(元/人)"]]
  ];
  for (const [name, headers] of sheets) {
    if (includedSheets && !includedSheets.includes(name)) continue;
    const sheet = workbook.addWorksheet(name);
    sheet.addRow(headers);
    sheet.addRow([marker]);
  }
  return Buffer.from(await workbook.xlsx.writeBuffer());
}
