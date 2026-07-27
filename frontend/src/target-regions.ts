import areaDataSource from "china-area-data/data.json";

type AreaData = Record<string, Record<string, string>>;
export type RegionGroup = { province: string; cities: string[] };
export type RegionState = { nationwide: boolean; groups: RegionGroup[]; unmatched: string[]; total: number };

const areaData = areaDataSource as AreaData;

function normalizedAreaName(value: string): string {
  return value
    .replace(/特别行政区$/, "")
    .replace(/壮族自治区$|回族自治区$|维吾尔自治区$|自治区$/, "")
    .replace(/省$|市$|地区$|盟$|自治州$|林区$|县$/, "");
}

export const provinceNames = Object.entries(areaData["86"] ?? {}).map(([code, name]) => ({
  code,
  mapName: normalizedAreaName(name)
}));

const cityProvince = new Map<string, string>();
for (const province of provinceNames) {
  cityProvince.set(province.mapName, province.mapName);
  for (const [cityCode, cityName] of Object.entries(areaData[province.code] ?? {})) {
    cityProvince.set(normalizedAreaName(cityName), province.mapName);
    if (cityName.includes("直辖县级行政区划")) {
      for (const countyName of Object.values(areaData[cityCode] ?? {})) {
        cityProvince.set(normalizedAreaName(countyName), province.mapName);
      }
    }
  }
}

for (const cityName of ["石河子", "阿拉尔", "图木舒克", "五家渠", "北屯", "铁门关", "双河", "可克达拉", "昆玉", "胡杨河", "新星", "白杨"]) {
  cityProvince.set(cityName, "新疆");
}

export function splitRegions(value: string): string[] {
  if (!value || value === "all" || value === "-1") return [];
  return value.split("#").map((item) => item.trim()).filter(Boolean);
}

export function groupRegions(city: string): RegionState {
  const nationwide = city === "all";
  const cities = splitRegions(city);
  const grouped = new Map<string, string[]>();
  const unmatched: string[] = [];

  for (const cityName of cities) {
    const province = cityProvince.get(normalizedAreaName(cityName));
    if (!province) {
      unmatched.push(cityName);
      continue;
    }
    const values = grouped.get(province) ?? [];
    values.push(cityName);
    grouped.set(province, values);
  }

  return {
    nationwide,
    groups: provinceNames
      .filter((province) => grouped.has(province.mapName))
      .map((province) => ({ province: province.mapName, cities: grouped.get(province.mapName) ?? [] })),
    unmatched,
    total: nationwide ? provinceNames.length : cities.length
  };
}

export function targetProvinceNames(city: string): string[] {
  const regions = groupRegions(city);
  if (regions.nationwide) return provinceNames.map((province) => province.mapName);
  return [...regions.groups.map((group) => group.province), ...regions.unmatched].sort((a, b) => a.localeCompare(b, "zh-CN"));
}
