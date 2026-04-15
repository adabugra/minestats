<script lang="ts">
  import * as echarts from 'echarts';
  import { onMount } from 'svelte';
  import { getEChartsTheme } from '../echartsTheme';

  export let series: { name: string; color?: string; data: [number, number | null][] }[] = [];
  export let dark = false;
  export let rangeKey = '6h';

  let el: HTMLDivElement;
  let chart: echarts.ECharts | null = null;
  let lastRangeKey = rangeKey;
  let lastSeriesRef = series;
  let animateNextSeriesUpdate = false;
  let zoomWindow: { start: number; end: number } = { start: 0, end: 100 };

  function escapeHtml(value: string) {
    return value.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
  }

  function valueAtOrBefore(rows: [number, number | null][], targetTs: number): number | null {
    let left = 0;
    let right = rows.length - 1;
    let found: number | null = null;

    while (left <= right) {
      const mid = Math.floor((left + right) / 2);
      const [ts, value] = rows[mid];
      if (ts <= targetTs) {
        found = typeof value === 'number' ? value : found;
        left = mid + 1;
      } else {
        right = mid - 1;
      }
    }

    return found;
  }

  function hexToRgba(hex: string, alpha: number) {
    const match = hex.replace('#', '');
    if (!/^[0-9a-fA-F]{3}([0-9a-fA-F]{3})?$/.test(match)) return hex;
    const value =
      match.length === 3
        ? match
            .split('')
            .map((ch) => ch + ch)
            .join('')
        : match;
    const int = Number.parseInt(value, 16);
    const r = (int >> 16) & 255;
    const g = (int >> 8) & 255;
    const b = int & 255;
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }

  function buildOption(animateTransition: boolean): echarts.EChartsOption {
    const theme = getEChartsTheme(dark);
    const peakOnline = Math.max(
      0,
      ...series.flatMap((entry) => entry.data.map(([, value]) => (typeof value === 'number' ? value : 0)))
    );

    return {
      ...theme,
      animation: animateTransition,
      animationDuration: animateTransition ? 350 : 0,
      animationDurationUpdate: animateTransition ? 450 : 0,
      animationEasingUpdate: 'cubicOut',
      legend: { show: false },
      tooltip: {
        ...(theme.tooltip ?? {}),
        trigger: 'axis',
        axisPointer: { type: 'cross' },
        formatter: (params: unknown) => {
          const points = Array.isArray(params) ? (params as { axisValue?: number }[]) : [params as { axisValue?: number }];
          const hoveredTs = Number(points[0]?.axisValue);
          if (!Number.isFinite(hoveredTs)) return '';

          const timeLabel = new Intl.DateTimeFormat('en-GB', {
            day: '2-digit',
            month: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false
          }).format(new Date(hoveredTs));

          const rows = series.map((entry, idx) => {
            const lineColor = entry.color ?? (theme.color as string[])[idx % (theme.color as string[]).length] ?? '#10b981';
            const value = valueAtOrBefore(entry.data, hoveredTs);
            return {
              name: entry.name,
              color: lineColor,
              value
            };
          });

          const total = rows.reduce((sum, row) => sum + (row.value ?? 0), 0);
          const lines = rows
            .map(
              (row) =>
                `<div style="display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:4px;">
                  <span style="display:inline-flex;align-items:center;gap:6px;color:${dark ? '#cbd5e1' : '#334155'};">
                    <span style="width:8px;height:8px;border-radius:999px;background:${row.color};display:inline-block;"></span>
                    ${escapeHtml(row.name)}
                  </span>
                  <strong style="color:${dark ? '#f8fafc' : '#0f172a'};font-family:monospace;">${row.value ?? '-'}</strong>
                </div>`
            )
            .join('');

          return `<div>
            <div style="font-weight:600;margin-bottom:2px;color:${dark ? '#f8fafc' : '#0f172a'};">${timeLabel}</div>
            ${lines}
            <div style="margin-top:6px;padding-top:6px;border-top:1px solid ${dark ? '#374151' : '#e2e8f0'};display:flex;justify-content:space-between;">
              <span style="color:${dark ? '#94a3b8' : '#64748b'};">Total</span>
              <strong style="color:${dark ? '#f8fafc' : '#0f172a'};font-family:monospace;">${total}</strong>
            </div>
          </div>`;
        }
      },
      grid: {
        left: '2%',
        right: '2%',
        top: '5%',
        bottom: '22%',
        containLabel: true
      },
      xAxis: {
        ...(theme.xAxis as object),
        type: 'time'
      },
      yAxis: {
        ...(theme.yAxis as object),
        type: 'value',
        min: 0,
        max: Math.max(Math.ceil(peakOnline * 1.2), 10)
      },
      dataZoom: [
        {
          type: 'slider',
          start: zoomWindow.start,
          end: zoomWindow.end,
          height: 30,
          bottom: 40,
          textStyle: {
            color: dark ? '#94a3b8' : '#64748b'
          },
          handleStyle: {
            color: '#10b981'
          },
          dataBackground: {
            areaStyle: {
              color: 'rgba(16, 185, 129, 0.2)'
            },
            lineStyle: {
              color: '#10b981'
            }
          }
        },
        {
          type: 'inside',
          start: zoomWindow.start,
          end: zoomWindow.end
        }
      ],
      series: series.map((entry, idx) => {
        const lineColor = entry.color ?? (theme.color as string[])[idx % (theme.color as string[]).length] ?? '#10b981';
        return {
          name: entry.name,
          type: 'line',
          smooth: true,
          symbol: 'none',
          sampling: 'lttb',
          lineStyle: {
            color: lineColor,
            width: 2.4
          },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: hexToRgba(lineColor, dark ? 0.22 : 0.18) },
              { offset: 1, color: hexToRgba(lineColor, 0.02) }
            ])
          },
          data: entry.data
        };
      })
    };
  }

  function render() {
    if (!chart) return;
    const rangeChanged = rangeKey !== lastRangeKey;
    if (rangeChanged) {
      animateNextSeriesUpdate = true;
      lastRangeKey = rangeKey;
    }

    const seriesChanged = series !== lastSeriesRef;
    const animateTransition = animateNextSeriesUpdate && seriesChanged;

    chart.setOption(buildOption(animateTransition), {
      notMerge: true,
      lazyUpdate: true,
      silent: true
    });
    if (animateTransition) animateNextSeriesUpdate = false;
    lastSeriesRef = series;
  }

  onMount(() => {
    chart = echarts.init(el);
    chart.on('datazoom', () => {
      if (!chart) return;
      const option = chart.getOption();
      const zoom = option.dataZoom?.[0] as { start?: number; end?: number } | undefined;
      if (!zoom) return;
      if (typeof zoom.start === 'number' && typeof zoom.end === 'number') {
        zoomWindow = { start: zoom.start, end: zoom.end };
      }
    });
    render();

    const resize = () => chart?.resize();
    window.addEventListener('resize', resize);

    return () => {
      window.removeEventListener('resize', resize);
      chart?.dispose();
      chart = null;
    };
  });

  $: if (chart) {
    series;
    dark;
    rangeKey;
    render();
  }
</script>

<div bind:this={el} class="chart-wrap"></div>
