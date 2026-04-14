import type { EChartsOption } from 'echarts';

export function getEChartsTheme(dark = false) {
  return {
    color: ['#10b981', '#3b82f6', '#f59e0b', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316', '#6366f1', '#a855f7', '#06b6d4'],
    backgroundColor: 'transparent',
    textStyle: {
      fontFamily: 'Inter, sans-serif',
      color: dark ? '#94a3b8' : '#64748b'
    },
    tooltip: {
      backgroundColor: dark ? 'rgba(15, 17, 26, 0.96)' : 'rgba(255, 255, 255, 0.98)',
      borderColor: dark ? '#2e3038' : '#e2e8f0',
      borderWidth: 1,
      textStyle: {
        color: dark ? '#e2e8f0' : '#1e293b',
        fontSize: 13
      },
      shadowBlur: 8,
      shadowColor: dark ? 'rgba(0, 0, 0, 0.4)' : 'rgba(15, 23, 42, 0.12)',
      shadowOffsetX: 0,
      shadowOffsetY: 2
    },
    xAxis: {
      axisLine: {
        lineStyle: {
          color: dark ? '#2e3038' : '#cbd5e1'
        }
      },
      splitLine: {
        lineStyle: {
          color: dark ? '#252732' : '#e2e8f0'
        }
      },
      axisLabel: {
        color: dark ? '#94a3b8' : '#64748b',
        fontSize: 12
      }
    },
    yAxis: {
      axisLine: {
        lineStyle: {
          color: dark ? '#2e3038' : '#cbd5e1'
        }
      },
      splitLine: {
        lineStyle: {
          color: dark ? '#252732' : '#e2e8f0'
        }
      },
      axisLabel: {
        color: dark ? '#94a3b8' : '#64748b',
        fontSize: 12
      }
    }
  } as const satisfies EChartsOption;
}
