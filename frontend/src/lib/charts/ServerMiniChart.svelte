<script lang="ts">
  import * as echarts from 'echarts';
  import { onMount } from 'svelte';
  import { getEChartsTheme } from '../echartsTheme';

  export let name = '';
  export let data: [number, number | null][] = [];
  export let dark = false;
  export let color = '#10b981';

  let el: HTMLDivElement;
  let chart: echarts.ECharts | null = null;

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

  function buildOption(): echarts.EChartsOption {
    const theme = getEChartsTheme(dark);
    const peak = Math.max(0, ...data.map(([, value]) => (typeof value === 'number' ? value : 0)));
    const yMax = Math.max(Math.ceil(peak * 1.2), 5);

    return {
      ...theme,
      animation: false,
      legend: { show: false },
      tooltip: {
        ...(theme.tooltip ?? {}),
        trigger: 'axis',
        axisPointer: { type: 'line' },
        formatter: (params: unknown) => {
          const first = (params as { value: [number, number] }[])[0];
          return `${first.value[1]} players`;
        }
      },
      grid: {
        top: 8,
        left: 30,
        right: 8,
        bottom: 8,
        containLabel: false
      },
      xAxis: {
        type: 'time',
        show: false
      },
      yAxis: {
        type: 'value',
        show: true,
        position: 'left',
        min: 0,
        max: yMax,
        splitNumber: 3,
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: {
          color: dark ? '#94a3b8' : '#64748b',
          fontSize: 10,
          margin: 4,
          formatter: (value: number) => (value % 1 === 0 ? `${value}` : '')
        }
      },
      series: [
        {
          name,
          type: 'line',
          smooth: true,
          showSymbol: false,
          sampling: 'lttb',
          lineStyle: {
            color,
            width: 2.2
          },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: hexToRgba(color, 0.3) },
              { offset: 1, color: hexToRgba(color, 0.06) }
            ])
          },
          data
        }
      ]
    };
  }

  function render() {
    if (!chart) return;
    chart.setOption(buildOption(), {
      notMerge: true,
      lazyUpdate: true,
      silent: true
    });
  }

  onMount(() => {
    chart = echarts.init(el);
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
    data;
    dark;
    color;
    name;
    render();
  }
</script>

<div bind:this={el} class="chart-wrap-sm"></div>
