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

import React, { useEffect, useState, useContext, useRef, useMemo } from 'react';
import {
  API,
  showError,
  showSuccess,
  renderQuota,
  copy,
  getQuotaPerUnit,
  getCurrencyConfig,
} from '../helpers';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import InvitationCard from '../components/topup/InvitationCard';
import TransferModal from '../components/topup/modals/TransferModal';

const InviteRewardPage = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [affLink, setAffLink] = useState('');
  const [openTransfer, setOpenTransfer] = useState(false);
  const [transferAmount, setTransferAmount] = useState(0);
  const [invitedUsers, setInvitedUsers] = useState([]);
  const affFetchedRef = useRef(false);

  const currencyConfig = useMemo(() => getCurrencyConfig(), []);
  const quotaPerUnit = getQuotaPerUnit();
  const affQuota = userState?.user?.aff_quota || 0;

  const minTransferAmount = currencyConfig.rate;
  const maxTransferAmount = quotaPerUnit > 0
    ? (affQuota / quotaPerUnit) * currencyConfig.rate
    : 0;

  const amountToQuota = (amount) =>
    Math.round((Number(amount) / currencyConfig.rate) * quotaPerUnit);

  const minAmountLabel = () =>
    `${currencyConfig.symbol}${Number.isFinite(minTransferAmount) ? minTransferAmount.toFixed(2) : '0.00'}`;

  const getUserQuota = async () => {
    const res = await API.get('/api/user/self');
    const { success, message, data } = res.data;
    if (success) {
      userDispatch({ type: 'login', payload: data });
    } else {
      showError(message);
    }
  };

  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      setAffLink(`${window.location.origin}/register?aff=${data}`);
    } else {
      showError(message);
    }
  };

  const transfer = async () => {
    const quota = amountToQuota(transferAmount);
    if (!Number.isFinite(quota) || quota < quotaPerUnit) {
      showError(t('划转金额最低为') + ' ' + minAmountLabel());
      return;
    }
    if (quota > affQuota) {
      showError(t('可用邀请额度不足'));
      return;
    }
    const res = await API.post('/api/user/aff_transfer', { quota });
    const { success, message } = res.data;
    if (success) {
      showSuccess(message);
      setOpenTransfer(false);
      getUserQuota();
    } else {
      showError(message);
    }
  };

  const handleAffLinkClick = async () => {
    await copy(affLink);
    showSuccess(t('邀请链接已复制到剪切板'));
  };

  useEffect(() => {
    getUserQuota();
    setTransferAmount(minTransferAmount);
  }, []);

  useEffect(() => {
    if (affFetchedRef.current) return;
    affFetchedRef.current = true;
    getAffLink();
  }, []);

  useEffect(() => {
    API.get('/api/user/invited')
      .then((res) => {
        if (res.data.success) {
          setInvitedUsers(res.data.data || []);
        }
      })
      .catch(() => {});
  }, []);

  return (
    <div className='mt-[60px] px-2'>
      <div className='w-full max-w-7xl mx-auto'>
        <InvitationCard
          t={t}
          userState={userState}
          renderQuota={renderQuota}
          setOpenTransfer={setOpenTransfer}
          affLink={affLink}
          handleAffLinkClick={handleAffLinkClick}
          invitedUsers={invitedUsers}
        />
        <TransferModal
          t={t}
          openTransfer={openTransfer}
          transfer={transfer}
          handleTransferCancel={() => setOpenTransfer(false)}
          currencySymbol={currencyConfig.symbol}
          minTransferAmount={minTransferAmount}
          maxTransferAmount={maxTransferAmount}
          transferAmount={transferAmount}
          setTransferAmount={setTransferAmount}
        />
      </div>
    </div>
  );
};

export default InviteRewardPage;
