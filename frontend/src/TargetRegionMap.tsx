import { useEffect, useMemo, useRef } from "react";
import { MapChart } from "echarts/charts";
import { TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import chinaMap from "china-map-data/china.js";
import { groupRegions, provinceNames, splitRegions } from "./target-regions";

echarts.use([MapChart, TooltipComponent, CanvasRenderer]);
echarts.registerMap("china-target-regions", chinaMap as never);

type TargetRegionMapProps = { city: string; areaCode: string };

export default function TargetRegionMap({ city, areaCode }: TargetRegionMapProps) {
  const chartRef = useRef<HTMLDivElement>(null);
  const regionState = useMemo(() => groupRegions(city), [city]);
  const fallbackCodes = useMemo(() => splitRegions(areaCode), [areaCode]);
  const selectedGroups = regionState.nationwide
    ? provinceNames.map((province) => ({ province: province.mapName, cities: [] }))
    : regionState.groups;
  const option = useMemo<EChartsCoreOption>(() => ({
    animationDuration: 360,
    tooltip: {
      trigger: "item",
      backgroundColor: "rgba(31, 37, 38, .94)",
      borderWidth: 0,
      padding: [7, 9],
      textStyle: { color: "#fff", fontSize: 10 },
      formatter: (params: { name?: string; value?: number }) => {
        if (!params.value) return `${params.name ?? ""}<br/>未投放`;
        return regionState.nationwide
          ? `${params.name ?? ""}<br/>全国投放`
          : `${params.name ?? ""}<br/>${params.value} 个城市`;
      }
    },
    series: [{
      type: "map",
      map: "china-target-regions",
      roam: false,
      selectedMode: false,
      top: 12,
      right: 12,
      bottom: 12,
      left: 12,
      label: { show: false },
      itemStyle: {
        areaColor: "#edf1ef",
        borderColor: "#fff",
        borderWidth: 1,
        shadowBlur: 3,
        shadowColor: "rgba(42, 76, 65, .12)"
      },
      emphasis: {
        label: { show: true, color: "#25312d", fontSize: 9 },
        itemStyle: { areaColor: "#7fb39f", borderColor: "#fff" }
      },
      data: selectedGroups.map((group) => ({
        name: group.province,
        value: regionState.nationwide ? 1 : group.cities.length,
        itemStyle: { areaColor: "#4b8a74" }
      }))
    }]
  }), [regionState.nationwide, selectedGroups]);

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = echarts.init(chartRef.current, undefined, { renderer: "canvas" });
    chart.setOption(option);
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(chartRef.current);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [option]);

  const hasNamedRegions = regionState.nationwide || regionState.total > 0;
  const displayTotal = regionState.nationwide ? "全国" : `${regionState.total} 个地域`;

  return <section className="target-region-section">
    <header>
      <div><strong>地域投放</strong><span>已覆盖区域统一高亮</span></div>
      <b>{displayTotal}</b>
    </header>
    <div className="target-region-layout">
      <div className="target-region-map" ref={chartRef} role="img" aria-label={`中国地域投放地图，${displayTotal}`} />
      <div className="target-region-list" aria-label="全部投放地域">
        {regionState.nationwide ? <div className="target-region-nationwide">全国投放，覆盖全部省级区域</div> : null}
        {regionState.groups.map((group) => <section key={group.province}>
          <strong>{group.province}<small>{group.cities.length}</small></strong>
          <div>{group.cities.map((cityName) => <span key={cityName}>{cityName}</span>)}</div>
        </section>)}
        {regionState.unmatched.length > 0 ? <section>
          <strong>其他地域<small>{regionState.unmatched.length}</small></strong>
          <div>{regionState.unmatched.map((cityName) => <span key={cityName}>{cityName}</span>)}</div>
        </section> : null}
        {!hasNamedRegions && fallbackCodes.length > 0 ? <section>
          <strong>地域编码<small>{fallbackCodes.length}</small></strong>
          <div>{fallbackCodes.map((code) => <span key={code}>{code}</span>)}</div>
        </section> : null}
        {!hasNamedRegions && fallbackCodes.length === 0 ? <div className="target-region-nationwide">未配置地域限制</div> : null}
      </div>
    </div>
  </section>;
}
