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
import React, { useState, useEffect, useMemo, useRef } from 'react';
import {
  Table,
  Badge,
  Typography,
  Toast,
  Empty,
  Button,
  Form,
  Tag,
  Modal,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { Coins } from 'lucide-react';
import { IconSearch } from '@douyinfe/semi-icons';
import { API, timestamp2string } from '../../helpers';
import { isAdmin, createCardProPagination } from '../../helpers/utils';
import CardPro from '../common/ui/CardPro';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;

const STATUS_CONFIG = {
  success: { type: 'success', key: '成功' },
  pending: { type: 'warning', key: '待支付' },
  failed: { type: 'danger', key: '失败' },
  expired: { type: 'danger', key: '已过期' },
};

const PAYMENT_METHOD_MAP = {
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  alipay: '支付宝',
  wxpay: '微信',
};

const TopupHistoryContent = ({ t, enabled = true, selfOnly = false }) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [topups, setTopups] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [keyword, setKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const userIsAdmin = useMemo(() => isAdmin(), []);
  const formApiRef = useRef(null);

  const loadTopups = async (currentPage, currentPageSize, currentKeyword) => {
    setLoading(true);
    try {
      const base = selfOnly
        ? '/api/user/topup/self'
        : userIsAdmin
          ? '/api/user/topup'
          : '/api/user/topup/self';
      const qs =
        `p=${currentPage}&page_size=${currentPageSize}` +
        (currentKeyword ? `&keyword=${encodeURIComponent(currentKeyword)}` : '');
      const endpoint = `${base}?${qs}`;
      const res = await API.get(endpoint);
      const { success, message, data } = res.data;
      if (success) {
        setTopups(data.items || []);
        setTotal(data.total || 0);
      } else {
        Toast.error({ content: message || t('加载失败') });
      }
    } catch (error) {
      Toast.error({ content: t('加载账单失败') });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!enabled) {
      return;
    }
    loadTopups(page, pageSize, keyword);
  }, [enabled, page, pageSize, keyword, selfOnly]);

  const handlePageChange = (currentPage) => {
    setPage(currentPage);
  };

  const handlePageSizeChange = (currentPageSize) => {
    setPageSize(currentPageSize);
    setPage(1);
  };

  const searchTopups = (targetPage) => {
    const values = formApiRef.current?.getValues() || {};
    const kw = (values.searchKeyword || '').trim();
    setSearching(true);
    setKeyword(kw);
    if (targetPage) setPage(targetPage);
    setSearching(false);
  };

  const handleReset = () => {
    if (!formApiRef.current) return;
    formApiRef.current.reset();
    setTimeout(() => {
      searchTopups(1);
    }, 100);
  };

  const handleAdminComplete = async (tradeNo) => {
    try {
      const res = await API.post('/api/user/topup/complete', {
        trade_no: tradeNo,
      });
      const { success, message } = res.data;
      if (success) {
        Toast.success({ content: t('补单成功') });
        await loadTopups(page, pageSize, keyword);
      } else {
        Toast.error({ content: message || t('补单失败') });
      }
    } catch (e) {
      Toast.error({ content: t('补单失败') });
    }
  };

  const confirmAdminComplete = (tradeNo) => {
    Modal.confirm({
      title: t('确认补单'),
      content: t('是否将该订单标记为成功并为用户入账？'),
      onOk: () => handleAdminComplete(tradeNo),
    });
  };

  const renderStatusBadge = (status) => {
    const config = STATUS_CONFIG[status] || { type: 'primary', key: status };
    return (
      <span className='flex items-center gap-2'>
        <Badge dot type={config.type} />
        <span>{t(config.key)}</span>
      </span>
    );
  };

  const renderPaymentMethod = (pm) => {
    const displayName = PAYMENT_METHOD_MAP[pm];
    return <Text>{displayName ? t(displayName) : pm || '-'}</Text>;
  };

  const isSubscriptionTopup = (record) => {
    const tradeNo = (record?.trade_no || '').toLowerCase();
    return Number(record?.amount || 0) === 0 && tradeNo.startsWith('sub');
  };

  const columns = useMemo(() => {
    const baseColumns = [
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (text) => <Text copyable>{text}</Text>,
      },
      {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        key: 'payment_method',
        render: renderPaymentMethod,
      },
      {
        title: t('充值额度'),
        dataIndex: 'amount',
        key: 'amount',
        render: (amount, record) => {
          if (isSubscriptionTopup(record)) {
            return (
              <Tag shape='circle' size='small'>
                {t('订阅套餐')}
              </Tag>
            );
          }
          return (
            <span className='flex items-center gap-1'>
              <Coins size={16} />
              <Text>{amount}</Text>
            </span>
          );
        },
      },
      {
        title: t('支付金额'),
        dataIndex: 'money',
        key: 'money',
        render: (money) => <Text type='danger'>¥{money.toFixed(2)}</Text>,
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        render: renderStatusBadge,
      },
    ];

    if (userIsAdmin && !selfOnly) {
      baseColumns.push({
        title: t('操作'),
        key: 'action',
        render: (_, record) => {
          const actions = [];
          if (record.status === 'pending') {
            actions.push(
              <Button
                key='complete'
                size='small'
                type='primary'
                theme='outline'
                onClick={() => confirmAdminComplete(record.trade_no)}
              >
                {t('补单')}
              </Button>,
            );
          }
          return actions.length > 0 ? <>{actions}</> : null;
        },
      });
    }

    baseColumns.push({
      title: t('创建时间'),
      dataIndex: 'create_time',
      key: 'create_time',
      render: (time) => timestamp2string(time),
    });

    return baseColumns;
  }, [keyword, page, pageSize, selfOnly, t, userIsAdmin]);

  return (
    <CardPro
      type='type1'
      descriptionArea={
        <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
          <div className='flex items-center text-blue-500'>
            <Coins size={16} className='mr-2' />
            <Text>{t('充值账单')}</Text>
          </div>
        </div>
      }
      actionsArea={
        <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
          <div className='w-full md:w-full lg:w-auto order-1 md:order-2'>
            <Form
              getFormApi={(api) => {
                formApiRef.current = api;
              }}
              onSubmit={() => searchTopups(1)}
              allowEmpty={true}
              autoComplete='off'
              layout='horizontal'
              trigger='change'
              stopValidateWithError={false}
              className='w-full md:w-auto order-1 md:order-2'
            >
              <div className='flex flex-col md:flex-row items-center gap-2 w-full md:w-auto'>
                <div className='relative w-full md:w-56'>
                  <Form.Input
                    field='searchKeyword'
                    prefix={<IconSearch />}
                    placeholder={t('订单号')}
                    showClear
                    pure
                    size='small'
                  />
                </div>
                <div className='flex gap-2 w-full md:w-auto'>
                  <Button
                    type='tertiary'
                    htmlType='submit'
                    loading={loading || searching}
                    className='flex-1 md:flex-initial md:w-auto'
                    size='small'
                  >
                    {t('查询')}
                  </Button>
                  <Button
                    type='tertiary'
                    onClick={handleReset}
                    className='flex-1 md:flex-initial md:w-auto'
                    size='small'
                  >
                    {t('重置')}
                  </Button>
                </div>
              </div>
            </Form>
          </div>
        </div>
      }
      paginationArea={createCardProPagination({
        currentPage: page,
        pageSize: pageSize,
        total: total,
        onPageChange: handlePageChange,
        onPageSizeChange: handlePageSizeChange,
        isMobile: isMobile,
        t: t,
      })}
      t={t}
    >
      <Table
        columns={columns}
        dataSource={topups}
        loading={loading}
        rowKey='id'
        pagination={false}
        size='small'
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无充值记录')}
            style={{ padding: 24 }}
          />
        }
      />
    </CardPro>
  );
};

export default TopupHistoryContent;
