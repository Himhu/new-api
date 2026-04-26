import React, { useState, useEffect, useCallback } from 'react';
import { Card, Button, Spin, Empty } from '@douyinfe/semi-ui';
import { RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { isAdmin } from '../../helpers';
import GroupStatusCard from './GroupStatusCard';
import GroupDetailPanel from './GroupDetailPanel';

const GroupMonitoringDashboard = () => {
  const { t } = useTranslation();
  const [groups, setGroups] = useState([]);
  const [historyMap, setHistoryMap] = useState({});
  const [periodMinutes, setPeriodMinutes] = useState(null);
  const [intervalMinutes, setIntervalMinutes] = useState(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [selectedGroup, setSelectedGroup] = useState(null);

  const fetchHistory = useCallback(async (groupList) => {
    if (!groupList || groupList.length === 0) return;
    const admin = isAdmin();
    const results = {};
    await Promise.all(
      groupList.map(async (group) => {
        try {
          const endpoint = admin
            ? `/api/monitoring/admin/groups/${encodeURIComponent(group.group_name)}/history`
            : `/api/monitoring/public/groups/${encodeURIComponent(group.group_name)}/history`;
          const res = await API.get(endpoint, { disableDuplicate: true });
          if (res.data.success) {
            results[group.group_name] = res.data.data || [];
            // Extract period/interval from any successful response
            if (res.data.period_minutes != null) {
              setPeriodMinutes(res.data.period_minutes);
            }
            if (res.data.aggregation_interval_minutes != null) {
              setIntervalMinutes(res.data.aggregation_interval_minutes);
            }
          }
        } catch {
          // ignore individual failures
        }
      })
    );
    setHistoryMap(results);
  }, []);

  const fetchGroups = useCallback(async (withHistory = false) => {
    try {
      const endpoint = isAdmin()
        ? '/api/monitoring/admin/groups'
        : '/api/monitoring/public/groups';
      const res = await API.get(endpoint, { disableDuplicate: true });
      const { success, data, message } = res.data;
      if (success) {
        const groupList = data || [];
        setGroups(groupList);
        if (withHistory) {
          fetchHistory(groupList);
        } else {
          // Clean stale entries from historyMap when groups change
          const currentNames = new Set(groupList.map(g => g.group_name));
          setHistoryMap(prev => {
            const cleaned = {};
            for (const key of Object.keys(prev)) {
              if (currentNames.has(key)) {
                cleaned[key] = prev[key];
              }
            }
            return cleaned;
          });
        }
      } else {
        showError(message || t('获取监控数据失败'));
      }
    } catch (error) {
      if (error?.response?.status === 403) {
        setGroups([]);
        setHistoryMap({});
      }
    } finally {
      setLoading(false);
    }
  }, [t, fetchHistory]);

  useEffect(() => {
    fetchGroups(true);
    const interval = setInterval(() => fetchGroups(false), 60000);
    return () => clearInterval(interval);
  }, [fetchGroups]);

  const handleRefresh = async () => {
    if (!isAdmin()) return;
    setRefreshing(true);
    try {
      const res = await API.post('/api/monitoring/admin/refresh');
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('刷新已触发，数据将在几秒后更新'));
        setTimeout(() => {
          fetchGroups(true);
          setRefreshing(false);
        }, 3000);
      } else {
        showError(message);
        setRefreshing(false);
      }
    } catch (error) {
      showError(t('刷新失败'));
      setRefreshing(false);
    }
  };

  const handleGroupClick = (group) => {
    setSelectedGroup(selectedGroup?.group_name === group.group_name ? null : group);
  };

  if (loading) {
    return (
      <div className='mt-[60px] px-2 flex items-center justify-center' style={{ minHeight: 300 }}>
        <Spin size='large' />
      </div>
    );
  }

  const noCallCount = groups.filter((g) => g.availability_rate == null || isNaN(g.availability_rate) || g.availability_rate < 0).length;
  const activeGroups = groups.filter((g) => g.availability_rate != null && !isNaN(g.availability_rate) && g.availability_rate >= 0);
  const onlineCount = activeGroups.filter((g) => g.is_online ?? g.online_channels > 0).length;
  const offlineCount = activeGroups.length - onlineCount;
  const updatedAt = groups.length > 0 && groups[0]?.updated_at
    ? formatTimeAgo(groups[0].updated_at, t)
    : null;

  return (
    <div className='mt-[60px] px-2' style={{ margin: '60px auto 0' }}>
      {/* Status Page Header */}
      <div
        style={{
          background: 'var(--semi-color-bg-0)',
          border: '1px solid var(--semi-color-border)',
          borderRadius: 12,
          padding: '24px 28px',
          marginBottom: 24,
          boxShadow: '0 2px 8px rgba(0,0,0,0.08), 0 1px 3px rgba(0,0,0,0.06)',
        }}
      >
        <div className='flex items-center flex-wrap' style={{ gap: 16 }}>
          {/* Stats */}
          <div className='flex items-center' style={{ gap: 12 }}>
            <span className='flex items-center' style={{ gap: 5, fontSize: 13, fontWeight: 500, color: 'var(--semi-color-text-1)' }}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--semi-color-success)', display: 'inline-block' }} />
              {onlineCount} {t('当前可用')}
            </span>
            <span className='flex items-center' style={{ gap: 5, fontSize: 13, fontWeight: 500, color: 'var(--semi-color-text-1)' }}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--semi-color-danger)', display: 'inline-block' }} />
              {offlineCount} {t('探测异常')}
            </span>
            <span className='flex items-center' style={{ gap: 5, fontSize: 13, fontWeight: 500, color: 'var(--semi-color-text-1)' }}>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--semi-color-text-2)', display: 'inline-block' }} />
              {noCallCount} {t('无调用')}
            </span>
            <span style={{ width: 1, height: 16, background: 'var(--semi-color-border)', display: 'inline-block' }} />
            <span style={{ fontSize: 13, color: 'var(--semi-color-text-2)' }}>
              {groups.length} {t('个分组')}
            </span>
          </div>

          {/* Right side: updated time + refresh */}
          <div className='flex items-center' style={{ marginLeft: 'auto', gap: 12 }}>
            {updatedAt && (
              <span style={{ fontSize: 12, color: 'var(--semi-color-text-2)' }}>
                {t('数据更新于')} {updatedAt}
              </span>
            )}
            {isAdmin() && (
              <Button
                icon={<RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} />}
                onClick={handleRefresh}
                loading={refreshing}
                disabled={refreshing}
                size='small'
                style={{
                  borderRadius: 6,
                }}
              >
                {t('刷新')}
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Cards */}
      {groups.length === 0 ? (
        <Card>
          <Empty
            title={t('暂无监控数据')}
            description={
              isAdmin()
                ? t('请在系统设置中配置监控分组')
                : t('管理员尚未启用监控功能')
            }
          />
        </Card>
      ) : (
        <>
          <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4 mb-4'>
            {groups.map((group) => (
              <GroupStatusCard
                key={group.group_name}
                group={group}
                isSelected={selectedGroup?.group_name === group.group_name}
                onClick={() => handleGroupClick(group)}
                history={historyMap[group.group_name]}
                periodMinutes={periodMinutes}
                intervalMinutes={intervalMinutes}
              />
            ))}
          </div>

          <GroupDetailPanel
            visible={!!selectedGroup}
            groupName={selectedGroup?.group_name}
            isAdmin={isAdmin()}
            onClose={() => setSelectedGroup(null)}
          />
        </>
      )}
    </div>
  );
};

function formatTimeAgo(timestamp, t) {
  if (!timestamp) return '--';
  const now = Math.floor(Date.now() / 1000);
  const diff = now - timestamp;
  if (diff < 60) return t('刚刚');
  if (diff < 3600) return Math.floor(diff / 60) + ' ' + t('分钟前');
  if (diff < 86400) return Math.floor(diff / 3600) + ' ' + t('小时前');
  return new Date(timestamp * 1000).toLocaleString();
}

export default GroupMonitoringDashboard;
