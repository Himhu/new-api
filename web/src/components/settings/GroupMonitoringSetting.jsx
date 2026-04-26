import React, { useEffect, useState } from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import SettingsGroupMonitoring from '../../pages/Setting/Operation/SettingsGroupMonitoring';
import { API, showError, toBoolean } from '../../helpers';

const GroupMonitoringSetting = () => {
  let [inputs, setInputs] = useState({
    'group_monitoring_setting.availability_period_minutes': 60,
    'group_monitoring_setting.cache_hit_period_minutes': 60,
    'group_monitoring_setting.aggregation_interval_minutes': 5,
  });

  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        if (typeof inputs[item.key] === 'boolean') {
          newInputs[item.key] = toBoolean(item.value);
        } else {
          newInputs[item.key] = item.value;
        }
      });
      setInputs(newInputs);
    } else {
      showError(message);
    }
  };

  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
    } catch (error) {
      showError('刷新失败');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <>
      <Spin spinning={loading} size='large'>
        <Card style={{ marginTop: '10px' }}>
          <SettingsGroupMonitoring options={inputs} refresh={onRefresh} />
        </Card>
      </Spin>
    </>
  );
};

export default GroupMonitoringSetting;
