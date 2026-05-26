/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import {
  Avatar,
  Typography,
  Card,
  Button,
  Input,
  Badge,
  Table,
  Tag,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Users,
  BarChart2,
  TrendingUp,
  Gift,
  Zap,
  Clock,
  Snowflake,
} from 'lucide-react';

import { renderQuota } from '../../helpers';

const { Text } = Typography;

const formatRemaining = (t, unlockAt, nowSec) => {
  if (!unlockAt || unlockAt <= 0) return '';
  const diff = unlockAt - nowSec;
  if (diff <= 0) return t('即将入账');
  const days = Math.floor(diff / 86400);
  const hours = Math.floor((diff % 86400) / 3600);
  const minutes = Math.floor((diff % 3600) / 60);
  if (days > 0) return `${days}${t('天')}${hours}${t('小时')}`;
  if (hours > 0) return `${hours}${t('小时')}${minutes}${t('分钟')}`;
  if (minutes > 0) return `${minutes}${t('分钟')}`;
  return `${diff}${t('秒')}`;
};

const InvitedUserList = ({ t, invitedUsers, nowSec }) => {
  if (!invitedUsers || invitedUsers.length === 0) {
    return (
      <Card className='!rounded-xl w-full' title={<Text type='tertiary'>{t('受邀用户')}</Text>}>
        <div className='text-center py-8'>
          <Text type='tertiary'>{t('暂无邀请记录')}</Text>
        </div>
      </Card>
    );
  }

  const columns = [
    {
      title: t('用户名'),
      dataIndex: 'display_name',
      render: (text) => (
        <Text className='text-sm font-medium'>{text}</Text>
      ),
    },
    {
      title: t('已入账'),
      dataIndex: 'settled_quota',
      width: 120,
      render: (val) => (
        <Text className='text-sm'>{renderQuota(val || 0)}</Text>
      ),
    },
    {
      title: t('冷冻中'),
      dataIndex: 'pending_quota',
      width: 200,
      render: (val, record) => {
        if (!val || val <= 0) {
          return <Text type='tertiary' className='text-sm'>-</Text>;
        }
        const remaining = formatRemaining(t, record?.earliest_unlock_at, nowSec);
        const count = record?.pending_count || 0;
        return (
          <div className='flex flex-col gap-1'>
            <Tag size='small' type='ghost' color='orange'>
              {renderQuota(val)}
            </Tag>
            {(remaining || count > 1) && (
              <div className='flex items-center gap-1'>
                <Clock size={11} className='text-orange-500' />
                <Text type='tertiary' className='!text-xs'>
                  {remaining}
                  {count > 1 ? ` · ${count}${t('笔')}` : ''}
                </Text>
              </div>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <Card
      className='!rounded-xl w-full'
      title={<Text type='tertiary'>{t('受邀用户')} ({invitedUsers.length})</Text>}
    >
      <Table
        columns={columns}
        dataSource={invitedUsers}
        rowKey='id'
        pagination={false}
        size='small'
      />
    </Card>
  );
};

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  setOpenTransfer,
  affLink,
  handleAffLinkClick,
  invitedUsers,
}) => {
  const [nowSec, setNowSec] = useState(() => Math.floor(Date.now() / 1000));

  useEffect(() => {
    const hasPending = (invitedUsers || []).some(
      (u) => (u?.pending_quota || 0) > 0 && (u?.earliest_unlock_at || 0) > 0,
    );
    if (!hasPending) return undefined;
    const timer = setInterval(() => {
      setNowSec(Math.floor(Date.now() / 1000));
    }, 60000);
    return () => clearInterval(timer);
  }, [invitedUsers]);

  const pendingQuota = userState?.user?.aff_pending_quota || 0;
  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      {/* 卡片头部 */}
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='green' className='mr-3 shadow-md'>
          <Gift size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('邀请奖励')}
          </Typography.Text>
          <div className='text-xs'>{t('邀请好友获得额外奖励')}</div>
        </div>
      </div>

      {/* 收益展示区域 */}
      <Card
        className='!rounded-xl w-full'
        cover={
          <div
            className='relative h-30'
            style={{
              '--palette-primary-darkerChannel': '0 75 80',
              backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
              backgroundSize: 'cover',
              backgroundPosition: 'center',
              backgroundRepeat: 'no-repeat',
            }}
          >
            {/* 标题和按钮 */}
            <div className='relative z-10 h-full flex flex-col justify-between p-4'>
              <div className='flex justify-between items-center'>
                <Text strong style={{ color: 'white', fontSize: '16px' }}>
                  {t('收益统计')}
                </Text>
                <Button
                  type='primary'
                  theme='solid'
                  size='small'
                  disabled={
                    !userState?.user?.aff_quota ||
                    userState?.user?.aff_quota <= 0
                  }
                  onClick={() => setOpenTransfer(true)}
                  className='!rounded-lg'
                >
                  <Zap size={12} className='mr-1' />
                  {t('划转到余额')}
                </Button>
              </div>

              {/* 统计数据 */}
              <div className='grid grid-cols-2 sm:grid-cols-4 gap-4 mt-4'>
                {/* 待使用收益 */}
                <div className='text-center'>
                  <div
                    className='text-base sm:text-2xl font-bold mb-2'
                    style={{ color: 'white' }}
                  >
                    {renderQuota(userState?.user?.aff_quota || 0)}
                  </div>
                  <div className='flex items-center justify-center text-sm'>
                    <TrendingUp
                      size={14}
                      className='mr-1'
                      style={{ color: 'rgba(255,255,255,0.8)' }}
                    />
                    <Text
                      style={{
                        color: 'rgba(255,255,255,0.8)',
                        fontSize: '12px',
                      }}
                    >
                      {t('待使用收益')}
                    </Text>
                  </div>
                </div>

                {/* 冷冻金额 */}
                <div className='text-center'>
                  <div
                    className='text-base sm:text-2xl font-bold mb-2'
                    style={{ color: 'white' }}
                  >
                    {renderQuota(pendingQuota)}
                  </div>
                  <div className='flex items-center justify-center text-sm'>
                    <Snowflake
                      size={14}
                      className='mr-1'
                      style={{ color: 'rgba(255,255,255,0.8)' }}
                    />
                    <Text
                      style={{
                        color: 'rgba(255,255,255,0.8)',
                        fontSize: '12px',
                      }}
                    >
                      {t('冷冻金额')}
                    </Text>
                  </div>
                </div>

                {/* 总收益 */}
                <div className='text-center'>
                  <div
                    className='text-base sm:text-2xl font-bold mb-2'
                    style={{ color: 'white' }}
                  >
                    {renderQuota(userState?.user?.aff_history_quota || 0)}
                  </div>
                  <div className='flex items-center justify-center text-sm'>
                    <BarChart2
                      size={14}
                      className='mr-1'
                      style={{ color: 'rgba(255,255,255,0.8)' }}
                    />
                    <Text
                      style={{
                        color: 'rgba(255,255,255,0.8)',
                        fontSize: '12px',
                      }}
                    >
                      {t('总收益')}
                    </Text>
                  </div>
                </div>

                {/* 邀请人数 */}
                <div className='text-center'>
                  <div
                    className='text-base sm:text-2xl font-bold mb-2'
                    style={{ color: 'white' }}
                  >
                    {userState?.user?.invited_count || 0}
                  </div>
                  <div className='flex items-center justify-center text-sm'>
                    <Users
                      size={14}
                      className='mr-1'
                      style={{ color: 'rgba(255,255,255,0.8)' }}
                    />
                    <Text
                      style={{
                        color: 'rgba(255,255,255,0.8)',
                        fontSize: '12px',
                      }}
                    >
                      {t('邀请人数')}
                    </Text>
                  </div>
                </div>
              </div>
            </div>
          </div>
        }
      >
        {/* 邀请链接部分 */}
        <Input
          value={affLink}
          readonly
          className='!rounded-lg'
          prefix={t('邀请链接')}
          suffix={
            <Button
              type='primary'
              theme='solid'
              onClick={handleAffLinkClick}
              icon={<Copy size={14} />}
              className='!rounded-lg'
            >
              {t('复制')}
            </Button>
          }
        />
      </Card>

      {/* 受邀用户 + 奖励说明 两栏布局 */}
      <div className='grid grid-cols-1 lg:grid-cols-3 gap-6 mt-4'>
        <div className='lg:col-span-2'>
          <InvitedUserList t={t} invitedUsers={invitedUsers} nowSec={nowSec} />
        </div>
        <div className='lg:col-span-1'>
          <Card
            className='!rounded-xl w-full'
            title={<Text type='tertiary'>{t('奖励说明')}</Text>}
          >
            <div className='space-y-3'>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('邀请好友注册后，好友每次充值您都可按比例获得返利')}
                </Text>
              </div>

              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('返利设有冷冻期，冷冻期结束后自动入账')}
                </Text>
              </div>

              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('邀请的好友越多，获得的奖励越多')}
                </Text>
              </div>
            </div>
          </Card>
        </div>
      </div>
    </Card>
  );
};

export default InvitationCard;
