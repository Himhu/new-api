import React, { useEffect, useMemo } from 'react';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { useTranslation } from 'react-i18next';

const AvailabilityCacheChart = ({ history, periodMinutes, intervalMinutes }) => {
  useEffect(() => {
    initVChartSemiTheme({
      isWatchingThemeSwitch: true,
    });
  }, []);
  const { t } = useTranslation();

  const chartData = useMemo(() => {
    if (!history || history.length === 0) return [];

    const stepSec = (intervalMinutes || 5) * 60;
    const periodSec = (periodMinutes || 1440) * 60;
    const now = Math.floor(Date.now() / 1000);

    const align = (ts) => Math.floor(ts / stepSec) * stepSec;
    const clampRate = (rate) => {
      if (rate == null || rate < 0) return null;
      return Math.min(parseFloat(rate.toFixed(2)), 100);
    };

    const dataMap = new Map();
    history.forEach((item) => {
      const key = align(item.recorded_at);
      const existing = dataMap.get(key);
      if (!existing || item.recorded_at > existing.recorded_at) {
        dataMap.set(key, item);
      }
    });

    const currentBucketStart = align(now);
    const gridStart = currentBucketStart - periodSec;
    const gridEnd = currentBucketStart;
    const slots = [];
    for (let ts = gridStart; ts <= gridEnd; ts += stepSec) {
      slots.push(ts);
    }

    const result = [];
    let lastAvailabilityValue = null;
    let lastCacheHitValue = null;
    let availabilityStarted = false;
    let cacheStarted = false;

    for (const ts of slots) {
      const item = dataMap.get(ts);
      const timeStr = new Date(ts * 1000).toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
      });
      const availabilityValue = clampRate(item?.availability_rate);
      const cacheHitValue = clampRate(item?.cache_hit_rate);

      if (availabilityValue != null) {
        lastAvailabilityValue = availabilityValue;
        availabilityStarted = true;
      }
      if (cacheHitValue != null) {
        lastCacheHitValue = cacheHitValue;
        cacheStarted = true;
      }

      result.push({
        time: timeStr,
        value: availabilityValue != null ? availabilityValue : availabilityStarted ? lastAvailabilityValue : null,
        rawValue: availabilityValue,
        type: t('可用率'),
      });
      result.push({
        time: timeStr,
        value: cacheHitValue != null ? cacheHitValue : cacheStarted ? lastCacheHitValue : null,
        rawValue: cacheHitValue,
        type: t('缓存命中率'),
      });
    }

    return result;
  }, [history, periodMinutes, intervalMinutes, t]);

  if (!history || history.length === 0) {
    return (
      <div className='h-64 flex items-center justify-center text-gray-400'>
        {t('暂无历史数据')}
      </div>
    );
  }

  const values = chartData.map((d) => d.value).filter((value) => value != null);
  const minValue = values.length > 0 ? Math.min(...values) : 0;
  const yMin = Math.max(0, Math.floor(minValue) - 20);

  const spec = {
    type: 'line',
    data: [
      {
        id: 'data',
        values: chartData,
      },
    ],
    xField: 'time',
    yField: 'value',
    seriesField: 'type',
    point: {
      visible: false,
    },
    line: {
      style: {
        curveType: 'monotone',
        lineWidth: 2,
      },
    },
    axes: [
      {
        orient: 'bottom',
        type: 'band',
        label: {
          autoRotate: true,
          style: {
            fontSize: 10,
          },
        },
      },
      {
        orient: 'left',
        type: 'linear',
        min: yMin,
        max: 100,
        title: {
          visible: true,
          text: '%',
        },
      },
    ],
    legends: {
      visible: true,
      orient: 'top',
    },
    tooltip: {
      visible: true,
      mark: {
        content: [
          {
            key: (datum) => datum.type,
            value: (datum) => (datum.rawValue == null ? t('暂无调用') : datum.rawValue + '%'),
          },
        ],
      },
    },
    color: {
      type: 'ordinal',
      domain: [t('可用率'), t('缓存命中率')],
      range: ['#3b82f6', '#14b8a6'],
    },
  };

  return (
    <div className='h-64'>
      <VChart spec={spec} option={{ mode: 'desktop-browser' }} />
    </div>
  );
};

export default AvailabilityCacheChart;
