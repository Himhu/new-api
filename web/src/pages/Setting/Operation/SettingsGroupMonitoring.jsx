import React, { useEffect, useState, useRef, useContext } from 'react';
import { Button, Col, Form, Row, Spin, Switch, Select, TagInput, Typography } from '@douyinfe/semi-ui';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../../context/Status';

export default function SettingsGroupMonitoring(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [statusState, statusDispatch] = useContext(StatusContext);
  const [inputs, setInputs] = useState({
    'group_monitoring_setting.availability_period_minutes': 60,
    'group_monitoring_setting.cache_hit_period_minutes': 60,
    'group_monitoring_setting.aggregation_interval_minutes': 5,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);
  const [availableGroups, setAvailableGroups] = useState([]);

  // Monitoring groups (managed outside form, merged with display order)
  const [monitoringGroups, setMonitoringGroups] = useState([]);
  const [monitoringGroupsOriginal, setMonitoringGroupsOriginal] = useState([]);

  // Sidebar and header nav monitoring toggle states
  const [sidebarEnabled, setSidebarEnabled] = useState(true);
  const [headerNavEnabled, setHeaderNavEnabled] = useState(true);
  const [sidebarEnabledOriginal, setSidebarEnabledOriginal] = useState(true);
  const [headerNavEnabledOriginal, setHeaderNavEnabledOriginal] = useState(true);
  const sidebarConfigRef = useRef(null);
  const headerNavConfigRef = useRef(null);

  // Fetch available groups for select options
  useEffect(() => {
    API.get('/api/group/').then((res) => {
      if (res.data.success && res.data.data) {
        const groups = res.data.data.map((g) => (typeof g === 'string' ? g : g.group));
        setAvailableGroups(groups.filter(Boolean));
      }
    }).catch(() => {});
  }, []);

  // Tag-based fields (managed outside form, saved as JSON arrays)
  const tagFieldKeys = [
    'group_monitoring_setting.availability_exclude_models',
    'group_monitoring_setting.cache_hit_exclude_models',
    'group_monitoring_setting.availability_exclude_keywords',
    'group_monitoring_setting.cache_tokens_separate_groups',
  ];
  const [tagFields, setTagFields] = useState({
    'group_monitoring_setting.availability_exclude_models': [],
    'group_monitoring_setting.cache_hit_exclude_models': [],
    'group_monitoring_setting.availability_exclude_keywords': [],
    'group_monitoring_setting.cache_tokens_separate_groups': [],
  });
  const [tagFieldsOriginal, setTagFieldsOriginal] = useState({
    'group_monitoring_setting.availability_exclude_models': [],
    'group_monitoring_setting.cache_hit_exclude_models': [],
    'group_monitoring_setting.availability_exclude_keywords': [],
    'group_monitoring_setting.cache_tokens_separate_groups': [],
  });

  function handleTagFieldChange(fieldName) {
    return (val) => {
      setTagFields((prev) => ({ ...prev, [fieldName]: val }));
    };
  }

  function parseJsonArray(val) {
    if (Array.isArray(val)) return val;
    if (typeof val === 'string') {
      try {
        const parsed = JSON.parse(val);
        if (Array.isArray(parsed)) return parsed;
      } catch (e) {
        // ignore
      }
    }
    return [];
  }

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    const sidebarChanged = sidebarEnabled !== sidebarEnabledOriginal;
    const headerNavChanged = headerNavEnabled !== headerNavEnabledOriginal;
    const groupsChanged = JSON.stringify(monitoringGroups) !== JSON.stringify(monitoringGroupsOriginal);
    const tagChanges = tagFieldKeys.filter((key) =>
      JSON.stringify(tagFields[key]) !== JSON.stringify(tagFieldsOriginal[key])
    );

    if (!updateArray.length && !sidebarChanged && !headerNavChanged && !groupsChanged && !tagChanges.length) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    const requestQueue = updateArray.map((item) => {
      let value = String(inputs[item.key]);
      return API.put('/api/option/', { key: item.key, value });
    });

    // Save tag fields that changed
    for (const key of tagChanges) {
      requestQueue.push(
        API.put('/api/option/', { key, value: JSON.stringify(tagFields[key]) })
      );
    }

    // Save monitoring groups (merged with display order)
    if (groupsChanged) {
      const groupsJson = JSON.stringify(monitoringGroups);
      requestQueue.push(
        API.put('/api/option/', {
          key: 'group_monitoring_setting.monitoring_groups',
          value: groupsJson,
        })
      );
      requestQueue.push(
        API.put('/api/option/', {
          key: 'group_monitoring_setting.group_display_order',
          value: groupsJson,
        })
      );
    }

    // Save sidebar config if changed
    if (sidebarChanged) {
      const config = sidebarConfigRef.current
        ? structuredClone(sidebarConfigRef.current)
        : {};
      if (!config.console) {
        config.console = { enabled: true };
      }
      config.console['group-monitoring'] = sidebarEnabled;
      sidebarConfigRef.current = config;
      requestQueue.push(
        API.put('/api/option/', {
          key: 'SidebarModulesAdmin',
          value: JSON.stringify(config),
        })
      );
    }

    // Save header nav config if changed
    if (headerNavChanged) {
      const config = headerNavConfigRef.current
        ? structuredClone(headerNavConfigRef.current)
        : {};
      config.monitoring = headerNavEnabled;
      headerNavConfigRef.current = config;
      requestQueue.push(
        API.put('/api/option/', {
          key: 'HeaderNavModules',
          value: JSON.stringify(config),
        })
      );
    }

    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (res.includes(undefined)) {
          return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));

        if (groupsChanged) {
          setMonitoringGroupsOriginal([...monitoringGroups]);
        }
        if (tagChanges.length) {
          setTagFieldsOriginal({ ...tagFields });
        }

        if (sidebarChanged || headerNavChanged) {
          const statusUpdate = { ...statusState.status };
          if (sidebarChanged) {
            statusUpdate.SidebarModulesAdmin = JSON.stringify(sidebarConfigRef.current);
            setSidebarEnabledOriginal(sidebarEnabled);
          }
          if (headerNavChanged) {
            statusUpdate.HeaderNavModules = JSON.stringify(headerNavConfigRef.current);
            setHeaderNavEnabledOriginal(headerNavEnabled);
          }
          statusDispatch({ type: 'set', payload: statusUpdate });
        }

        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  function handleRefresh() {
    setRefreshing(true);
    API.post('/api/monitoring/admin/refresh')
      .then((res) => {
        if (res.data.success) {
          showSuccess(t('刷新已触发，数据将在几秒后更新'));
        } else {
          showError(res.data.message);
        }
      })
      .catch(() => {
        showError(t('刷新失败'));
      })
      .finally(() => {
        setRefreshing(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);

    // Load tag fields
    const newTagFields = {};
    for (const key of tagFieldKeys) {
      newTagFields[key] = props.options[key] ? parseJsonArray(props.options[key]) : [];
    }
    setTagFields(newTagFields);
    setTagFieldsOriginal(structuredClone(newTagFields));

    // Load monitoring groups
    const mgKey = 'group_monitoring_setting.monitoring_groups';
    if (props.options[mgKey]) {
      const groups = parseJsonArray(props.options[mgKey]);
      setMonitoringGroups(groups);
      setMonitoringGroupsOriginal([...groups]);
    }

    // Parse sidebar admin config
    if (props.options.SidebarModulesAdmin) {
      try {
        const config = JSON.parse(props.options.SidebarModulesAdmin);
        sidebarConfigRef.current = config;
        const enabled = config?.console?.['group-monitoring'] !== false;
        setSidebarEnabled(enabled);
        setSidebarEnabledOriginal(enabled);
      } catch (e) {
        sidebarConfigRef.current = null;
      }
    }

    // Parse header nav config
    if (props.options.HeaderNavModules) {
      try {
        const config = JSON.parse(props.options.HeaderNavModules);
        headerNavConfigRef.current = config;
        const enabled = typeof config?.monitoring === 'boolean'
          ? config.monitoring
          : true;
        setHeaderNavEnabled(enabled);
        setHeaderNavEnabledOriginal(enabled);
      } catch (e) {
        headerNavConfigRef.current = null;
      }
    }
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
          onSubmit={() => {}}
        >
          <Form.Section text={t('分组监控设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('控制台分组监控')}
                  </div>
                  <Switch
                    checked={sidebarEnabled}
                    onChange={(value) => setSidebarEnabled(value)}
                    size='default'
                    checkedText='｜'
                    uncheckedText='〇'
                  />
                  <Typography.Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
                    {t('控制侧边栏是否显示分组监控入口')}
                  </Typography.Text>
                </div>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('顶栏导航分组监控')}
                  </div>
                  <Switch
                    checked={headerNavEnabled}
                    onChange={(value) => setHeaderNavEnabled(value)}
                    size='default'
                    checkedText='｜'
                    uncheckedText='〇'
                  />
                  <Typography.Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
                    {t('控制顶栏导航是否显示分组监控入口')}
                  </Typography.Text>
                </div>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16} md={12}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('监控分组列表')}
                  </div>
                  <Select
                    filter
                    optionList={availableGroups.filter((g) => !monitoringGroups.includes(g)).map((g) => ({ value: g, label: g }))}
                    onChange={(value) => {
                      if (value && !monitoringGroups.includes(value)) {
                        setMonitoringGroups([...monitoringGroups, value]);
                      }
                    }}
                    placeholder={t('选择添加监控分组')}
                    style={{ width: '100%', marginBottom: 8 }}
                    value={undefined}
                  />
                  <TagInput
                    draggable
                    value={monitoringGroups}
                    onChange={(val) => setMonitoringGroups(val)}
                    placeholder={t('拖拽标签调整显示排序')}
                    allowDuplicates={false}
                  />
                </div>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('可用率统计周期')}
                  step={5}
                  min={5}
                  suffix={t('分钟')}
                  extraText={t('统计多长时间内的日志来计算可用率')}
                  field={'group_monitoring_setting.availability_period_minutes'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'group_monitoring_setting.availability_period_minutes': parseInt(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('缓存命中率统计周期')}
                  step={5}
                  min={5}
                  suffix={t('分钟')}
                  extraText={t('统计多长时间内的日志来计算缓存命中率')}
                  field={'group_monitoring_setting.cache_hit_period_minutes'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'group_monitoring_setting.cache_hit_period_minutes': parseInt(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('聚合间隔')}
                  step={1}
                  min={1}
                  suffix={t('分钟')}
                  extraText={t('每隔多少分钟聚合一次监控数据')}
                  field={'group_monitoring_setting.aggregation_interval_minutes'}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'group_monitoring_setting.aggregation_interval_minutes': parseInt(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('可用率排除模型')}
                  </div>
                  <TagInput
                    value={tagFields['group_monitoring_setting.availability_exclude_models']}
                    onChange={handleTagFieldChange('group_monitoring_setting.availability_exclude_models')}
                    placeholder={t('输入后按回车添加')}
                  />
                  <Typography.Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
                    {t('这些模型不纳入可用率计算')}
                  </Typography.Text>
                </div>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('缓存命中率排除模型')}
                  </div>
                  <TagInput
                    value={tagFields['group_monitoring_setting.cache_hit_exclude_models']}
                    onChange={handleTagFieldChange('group_monitoring_setting.cache_hit_exclude_models')}
                    placeholder={t('输入后按回车添加')}
                  />
                  <Typography.Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
                    {t('这些模型不纳入缓存命中率计算')}
                  </Typography.Text>
                </div>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('可用率排除关键词')}
                  </div>
                  <TagInput
                    value={tagFields['group_monitoring_setting.availability_exclude_keywords']}
                    onChange={handleTagFieldChange('group_monitoring_setting.availability_exclude_keywords')}
                    placeholder={t('输入后按回车添加')}
                  />
                  <Typography.Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
                    {t('包含这些关键词的错误不计入可用率')}
                  </Typography.Text>
                </div>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>
                    {t('缓存独立计算分组')}
                  </div>
                  <TagInput
                    value={tagFields['group_monitoring_setting.cache_tokens_separate_groups']}
                    onChange={handleTagFieldChange('group_monitoring_setting.cache_tokens_separate_groups')}
                    placeholder={t('输入后按回车添加')}
                  />
                  <Typography.Text type='tertiary' size='small' style={{ display: 'block', marginTop: 4 }}>
                    {t('这些分组的 prompt_tokens 不含缓存（如 Claude API），使用 cache/(prompt+cache) 公式')}
                  </Typography.Text>
                </div>
              </Col>
            </Row>
            <Row gutter={16} style={{ marginTop: 16 }}>
              <Col>
                <Button size='default' onClick={onSubmit}>
                  {t('保存分组监控设置')}
                </Button>
                <Button
                  size='default'
                  type='secondary'
                  style={{ marginLeft: 8 }}
                  loading={refreshing}
                  onClick={handleRefresh}
                >
                  {t('立即刷新数据')}
                </Button>
              </Col>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
