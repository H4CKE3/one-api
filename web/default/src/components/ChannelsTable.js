import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Dropdown,
  Form,
  Input,
  Label,
  Message,
  Modal,
  Pagination,
  Popup,
  Table,
} from 'semantic-ui-react';
import { Link } from 'react-router-dom';
import {
  API,
  loadChannelModels,
  setPromptShown,
  shouldShowPrompt,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../helpers';

import { CHANNEL_OPTIONS, ITEMS_PER_PAGE } from '../constants';
import { renderGroup, renderNumber } from '../helpers/render';

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

let type2label = undefined;

function renderType(type, t) {
  if (!type2label) {
    type2label = new Map();
    for (let i = 0; i < CHANNEL_OPTIONS.length; i++) {
      type2label[CHANNEL_OPTIONS[i].value] = CHANNEL_OPTIONS[i];
    }
    type2label[0] = {
      value: 0,
      text: t('channel.table.status_unknown'),
      color: 'grey',
    };
  }
  return (
    <Label basic color={type2label[type]?.color}>
      {type2label[type] ? type2label[type].text : type}
    </Label>
  );
}

function renderBalance(type, balance, t) {
  switch (type) {
    case 1: // OpenAI
      if (balance === 0) {
        return <span>{t('channel.table.balance_not_supported')}</span>;
      }
      return <span>${balance.toFixed(2)}</span>;
    case 4: // CloseAI
      return <span>¥{balance.toFixed(2)}</span>;
    case 8: // 自定义
      return <span>${balance.toFixed(2)}</span>;
    case 5: // OpenAI-SB
      return <span>¥{(balance / 10000).toFixed(2)}</span>;
    case 10: // AI Proxy
      return <span>{renderNumber(balance)}</span>;
    case 12: // API2GPT
      return <span>¥{balance.toFixed(2)}</span>;
    case 13: // AIGC2D
      return <span>{renderNumber(balance)}</span>;
    case 20: // OpenRouter
      return <span>${balance.toFixed(2)}</span>;
    case 36: // DeepSeek
      return <span>¥{balance.toFixed(2)}</span>;
    case 44: // SiliconFlow
      return <span>¥{balance.toFixed(2)}</span>;
    default:
      return <span>{t('channel.table.balance_not_supported')}</span>;
  }
}

function isShowDetail() {
  return localStorage.getItem('show_detail') === 'true';
}

const promptID = 'detail';

const ChannelsTable = () => {
  const { t } = useTranslation();
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [updatingBalance, setUpdatingBalance] = useState(false);
  const [showPrompt, setShowPrompt] = useState(shouldShowPrompt(promptID));
  const [showDetail, setShowDetail] = useState(isShowDetail());

  // 错误记录相关状态
  const [errorRecordsModalOpen, setErrorRecordsModalOpen] = useState(false);
  const [selectedChannelId, setSelectedChannelId] = useState(null);
  const [selectedChannelName, setSelectedChannelName] = useState('');
  const [errorRecords, setErrorRecords] = useState([]);
  const [errorRecordsLoading, setErrorRecordsLoading] = useState(false);
  const [errorRecordsPage, setErrorRecordsPage] = useState(1);
  const [errorRecordsTotal, setErrorRecordsTotal] = useState(0);
  const [detailModalOpen, setDetailModalOpen] = useState(false);
  const [selectedRecord, setSelectedRecord] = useState(null);
  const [conversationRecords, setConversationRecords] = useState([]);
  const [conversationLoading, setConversationLoading] = useState(false);

  const processChannelData = (channel) => {
    if (channel.models === '') {
      channel.models = [];
      channel.test_model = '';
    } else {
      channel.models = channel.models.split(',');
      if (channel.models.length > 0) {
        channel.test_model = channel.models[0];
      }
      channel.model_options = channel.models.map((model) => {
        return {
          key: model,
          text: model,
          value: model,
        };
      });
      console.log('channel', channel);
    }
    return channel;
  };

  const loadChannels = async (startIdx) => {
    const res = await API.get(`/api/channel/?p=${startIdx}`);
    const { success, message, data } = res.data;
    if (success) {
      let localChannels = data.map(processChannelData);
      if (startIdx === 0) {
        setChannels(localChannels);
      } else {
        let newChannels = [...channels];
        newChannels.splice(
          startIdx * ITEMS_PER_PAGE,
          data.length,
          ...localChannels
        );
        setChannels(newChannels);
      }
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      if (activePage === Math.ceil(channels.length / ITEMS_PER_PAGE) + 1) {
        // In this case we have to load more data and then append them.
        await loadChannels(activePage - 1);
      }
      setActivePage(activePage);
    })();
  };

  const refresh = async () => {
    setLoading(true);
    await loadChannels(activePage - 1);
  };

  const toggleShowDetail = () => {
    setShowDetail(!showDetail);
    localStorage.setItem('show_detail', (!showDetail).toString());
  };

  useEffect(() => {
    loadChannels(0)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    loadChannelModels().then();
  }, []);

  const manageChannel = async (id, action, idx, value) => {
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/channel/${id}/`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/channel/', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/channel/', data);
        break;
      case 'priority':
        if (value === '') {
          return;
        }
        data.priority = parseInt(value);
        res = await API.put('/api/channel/', data);
        break;
      case 'weight':
        if (value === '') {
          return;
        }
        data.weight = parseInt(value);
        if (data.weight < 0) {
          data.weight = 0;
        }
        res = await API.put('/api/channel/', data);
        break;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('channel.messages.operation_success'));
      let channel = res.data.data;
      let newChannels = [...channels];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      if (action === 'delete') {
        newChannels[realIdx].deleted = true;
      } else {
        newChannels[realIdx].status = channel.status;
      }
      setChannels(newChannels);
    } else {
      showError(message);
    }
  };

  const renderStatus = (status, t) => {
    switch (status) {
      case 1:
        return (
          <Label basic color='green'>
            {t('channel.table.status_enabled')}
          </Label>
        );
      case 2:
        return (
          <Popup
            trigger={
              <Label basic color='red'>
                {t('channel.table.status_disabled')}
              </Label>
            }
            content={t('channel.table.status_disabled_tip')}
            basic
          />
        );
      case 3:
        return (
          <Popup
            trigger={
              <Label basic color='yellow'>
                {t('channel.table.status_auto_disabled')}
              </Label>
            }
            content={t('channel.table.status_auto_disabled_tip')}
            basic
          />
        );
      default:
        return (
          <Label basic color='grey'>
            {t('channel.table.status_unknown')}
          </Label>
        );
    }
  };

  const renderResponseTime = (responseTime, t) => {
    let time = responseTime / 1000;
    time = time.toFixed(2) + 's';
    if (responseTime === 0) {
      return (
        <Label basic color='grey'>
          {t('channel.table.not_tested')}
        </Label>
      );
    } else if (responseTime <= 1000) {
      return (
        <Label basic color='green'>
          {time}
        </Label>
      );
    } else if (responseTime <= 3000) {
      return (
        <Label basic color='olive'>
          {time}
        </Label>
      );
    } else if (responseTime <= 5000) {
      return (
        <Label basic color='yellow'>
          {time}
        </Label>
      );
    } else {
      return (
        <Label basic color='red'>
          {time}
        </Label>
      );
    }
  };

  const searchChannels = async () => {
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadChannels(0);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await API.get(`/api/channel/search?keyword=${searchKeyword}`);
    const { success, message, data } = res.data;
    if (success) {
      let localChannels = data.map(processChannelData);
      setChannels(localChannels);
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const switchTestModel = async (idx, model) => {
    let newChannels = [...channels];
    let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
    newChannels[realIdx].test_model = model;
    setChannels(newChannels);
  };

  const testChannel = async (id, name, idx, m) => {
    const res = await API.get(`/api/channel/test/${id}?model=${m}`);
    const { success, message, time, model } = res.data;
    if (success) {
      let newChannels = [...channels];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      newChannels[realIdx].response_time = time * 1000;
      newChannels[realIdx].test_time = Date.now() / 1000;
      setChannels(newChannels);
      showSuccess(
        t('channel.messages.test_success', { name, model, time, message })
      );
    } else {
      showError(message);
    }
    let newChannels = [...channels];
    let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
    newChannels[realIdx].response_time = time * 1000;
    newChannels[realIdx].test_time = Date.now() / 1000;
    setChannels(newChannels);
  };

  const testChannels = async (scope) => {
    const res = await API.get(`/api/channel/test?scope=${scope}`);
    const { success, message } = res.data;
    if (success) {
      showInfo(t('channel.messages.test_all_started'));
    } else {
      showError(message);
    }
  };

  const deleteAllDisabledChannels = async () => {
    const res = await API.delete(`/api/channel/disabled`);
    const { success, message, data } = res.data;
    if (success) {
      showSuccess(
        t('channel.messages.delete_disabled_success', { count: data })
      );
      await refresh();
    } else {
      showError(message);
    }
  };

  const updateChannelBalance = async (id, name, idx) => {
    const res = await API.get(`/api/channel/update_balance/${id}/`);
    const { success, message, balance } = res.data;
    if (success) {
      let newChannels = [...channels];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      newChannels[realIdx].balance = balance;
      newChannels[realIdx].balance_updated_time = Date.now() / 1000;
      setChannels(newChannels);
      showSuccess(t('channel.messages.balance_update_success', { name }));
    } else {
      showError(message);
    }
  };

  const updateAllChannelsBalance = async () => {
    setUpdatingBalance(true);
    const res = await API.get(`/api/channel/update_balance`);
    const { success, message } = res.data;
    if (success) {
      showInfo(t('channel.messages.all_balance_updated'));
    } else {
      showError(message);
    }
    setUpdatingBalance(false);
  };

  const handleKeywordChange = async (e, { value }) => {
    setSearchKeyword(value.trim());
  };

  const sortChannel = (key) => {
    if (channels.length === 0) return;
    setLoading(true);
    let sortedChannels = [...channels];
    sortedChannels.sort((a, b) => {
      if (!isNaN(a[key])) {
        // If the value is numeric, subtract to sort
        return a[key] - b[key];
      } else {
        // If the value is not numeric, sort as strings
        return ('' + a[key]).localeCompare(b[key]);
      }
    });
    if (sortedChannels[0].id === channels[0].id) {
      sortedChannels.reverse();
    }
    setChannels(sortedChannels);
    setLoading(false);
  };

  // 获取渠道错误记录
  const loadErrorRecords = async (channelId, page) => {
    setErrorRecordsLoading(true);
    try {
      const res = await API.get(
        `/api/channel/${channelId}/error_records?p=${page - 1}`
      );
      const { success, message, data, total } = res.data;
      if (success) {
        setErrorRecords(data || []);
        setErrorRecordsTotal(total || 0);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('加载错误记录失败');
    }
    setErrorRecordsLoading(false);
  };

  // 打开错误记录弹窗
  const openErrorRecordsModal = (channelId, channelName) => {
    setSelectedChannelId(channelId);
    setSelectedChannelName(channelName);
    setErrorRecordsPage(1);
    setErrorRecordsModalOpen(true);
    loadErrorRecords(channelId, 1);
  };

  // 关闭错误记录弹窗
  const closeErrorRecordsModal = () => {
    setErrorRecordsModalOpen(false);
    setSelectedChannelId(null);
    setSelectedChannelName('');
    setErrorRecords([]);
  };

  // 获取会话所有记录
  const loadConversationRecords = async (conversationId) => {
    setConversationLoading(true);
    try {
      const res = await API.get(`/api/channel/conversation/${conversationId}`);
      const { success, message, data } = res.data;
      if (success) {
        setConversationRecords(data || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('加载会话记录失败');
    }
    setConversationLoading(false);
  };

  // 打开详情弹窗
  const openDetailModal = (record) => {
    setSelectedRecord(record);
    setDetailModalOpen(true);
    // 加载该会话的所有记录
    if (record.conversation_id) {
      loadConversationRecords(record.conversation_id);
    }
  };

  // 关闭详情弹窗
  const closeDetailModal = () => {
    setDetailModalOpen(false);
    setSelectedRecord(null);
    setConversationRecords([]);
  };

  // 错误记录分页变化
  const onErrorRecordsPageChange = (e, { activePage }) => {
    setErrorRecordsPage(activePage);
    loadErrorRecords(selectedChannelId, activePage);
  };

  return (
    <>
      {/* 错误记录列表弹窗 */}
      <Modal
        open={errorRecordsModalOpen}
        onClose={closeErrorRecordsModal}
        size='large'
      >
        <Modal.Header>渠道错误记录 - {selectedChannelName}</Modal.Header>
        <Modal.Content scrolling>
          {errorRecordsLoading ? (
            <Message>加载中...</Message>
          ) : errorRecords.length === 0 ? (
            <Message>暂无错误记录</Message>
          ) : (
            <Table celled>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell>ID</Table.HeaderCell>
                  <Table.HeaderCell>用户ID</Table.HeaderCell>
                  <Table.HeaderCell>模型</Table.HeaderCell>
                  <Table.HeaderCell>错误信息</Table.HeaderCell>
                  <Table.HeaderCell>创建时间</Table.HeaderCell>
                  <Table.HeaderCell>操作</Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {errorRecords.map((record) => (
                  <Table.Row key={record.id}>
                    <Table.Cell>{record.id}</Table.Cell>
                    <Table.Cell>{record.user_id}</Table.Cell>
                    <Table.Cell>{record.model}</Table.Cell>
                    <Table.Cell>
                      {record.error_message || '无错误信息'}
                    </Table.Cell>
                    <Table.Cell>
                      {timestamp2string(record.created_time)}
                    </Table.Cell>
                    <Table.Cell>
                      <Button
                        size='tiny'
                        primary
                        onClick={() => openDetailModal(record)}
                      >
                        详情
                      </Button>
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
              <Table.Footer>
                <Table.Row>
                  <Table.HeaderCell colSpan='6'>
                    <Pagination
                      floated='right'
                      activePage={errorRecordsPage}
                      onPageChange={onErrorRecordsPageChange}
                      size='tiny'
                      totalPages={Math.ceil(errorRecordsTotal / ITEMS_PER_PAGE)}
                    />
                    <span>总计: {errorRecordsTotal} 条错误记录</span>
                  </Table.HeaderCell>
                </Table.Row>
              </Table.Footer>
            </Table>
          )}
        </Modal.Content>
        <Modal.Actions>
          <Button onClick={closeErrorRecordsModal}>关闭</Button>
        </Modal.Actions>
      </Modal>

      {/* 错误记录详情弹窗 */}
      <Modal open={detailModalOpen} onClose={closeDetailModal} size='large'>
        <Modal.Header>
          会话详情 - {selectedRecord?.conversation_id}
        </Modal.Header>
        <Modal.Content scrolling>
          {selectedRecord && (
            <div>
              {/* 基本信息 */}
              <div
                style={{
                  padding: '15px',
                  background: '#f8f9fa',
                  borderRadius: '6px',
                  marginBottom: '20px',
                }}
              >
                <h4 style={{ marginTop: 0 }}>基本信息</h4>
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '1fr 1fr',
                    gap: '10px',
                    fontSize: '14px',
                  }}
                >
                  <div>
                    <strong>记录ID:</strong> {selectedRecord.id}
                  </div>
                  <div>
                    <strong>用户ID:</strong> {selectedRecord.user_id}
                  </div>
                  <div>
                    <strong>Token ID:</strong> {selectedRecord.token_id}
                  </div>
                  <div>
                    <strong>会话ID:</strong> {selectedRecord.conversation_id}
                  </div>
                  <div>
                    <strong>模型:</strong> {selectedRecord.model}
                  </div>
                  <div>
                    <strong>渠道:</strong> {selectedRecord.channel_name} (ID:{' '}
                    {selectedRecord.channel_id})
                  </div>
                  <div>
                    <strong>API类型:</strong> {selectedRecord.api_type}
                  </div>
                  <div>
                    <strong>请求ID:</strong> {selectedRecord.request_id}
                  </div>
                  <div>
                    <strong>Prompt Tokens:</strong>{' '}
                    {selectedRecord.prompt_tokens}
                  </div>
                  <div>
                    <strong>Completion Tokens:</strong>{' '}
                    {selectedRecord.completion_tokens}
                  </div>
                  <div>
                    <strong>Total Tokens:</strong> {selectedRecord.total_tokens}
                  </div>
                  <div>
                    <strong>消耗额度:</strong> {selectedRecord.cost}
                  </div>
                  <div>
                    <strong>响应时间:</strong> {selectedRecord.response_time}ms
                  </div>
                  <div>
                    <strong>状态:</strong>
                    <Label
                      size='mini'
                      color={selectedRecord.status === 1 ? 'green' : 'red'}
                      style={{ marginLeft: '5px' }}
                    >
                      {selectedRecord.status === 1 ? '成功' : '失败'}
                    </Label>
                  </div>
                  <div>
                    <strong>创建时间:</strong>{' '}
                    {timestamp2string(selectedRecord.created_time)}
                  </div>
                  <div>
                    <strong>更新时间:</strong>{' '}
                    {timestamp2string(selectedRecord.updated_time)}
                  </div>
                </div>

                {/* 错误信息 */}
                {selectedRecord.error_message && (
                  <div style={{ marginTop: '15px' }}>
                    <h4>错误信息</h4>
                    <div
                      style={{
                        padding: '10px',
                        background: '#fff3f3',
                        borderRadius: '4px',
                        fontFamily: 'monospace',
                        fontSize: '12px',
                        whiteSpace: 'pre-wrap',
                        wordBreak: 'break-word',
                        border: '1px solid #ffcdd2',
                      }}
                    >
                      {selectedRecord.error_message}
                    </div>
                  </div>
                )}
              </div>

              {/* 会话记录 */}
              <div>
                <h4>完整会话记录</h4>
                {conversationLoading ? (
                  <Message>加载中...</Message>
                ) : conversationRecords.length === 0 ? (
                  <Message>暂无会话记录</Message>
                ) : (
                  <div style={{ marginTop: '10px' }}>
                    {conversationRecords.map((record, index) => {
                      const isUser = record.role === 'user';
                      const isSystem = record.role === 'system';
                      const isError = record.status === 2;

                      return (
                        <div
                          key={record.id}
                          style={{
                            marginBottom: '15px',
                            padding: '12px',
                            borderRadius: '8px',
                            background: isSystem
                              ? '#e3f2fd'
                              : isUser
                              ? '#f5f5f5'
                              : isError
                              ? '#ffebee'
                              : '#e8f5e9',
                            border: isError
                              ? '2px solid #ef5350'
                              : '1px solid #ddd',
                            position: 'relative',
                          }}
                        >
                          {/* 角色标签 */}
                          <div
                            style={{
                              marginBottom: '8px',
                              display: 'flex',
                              alignItems: 'center',
                              gap: '8px',
                            }}
                          >
                            <Label
                              size='small'
                              color={
                                isSystem
                                  ? 'blue'
                                  : isUser
                                  ? 'grey'
                                  : isError
                                  ? 'red'
                                  : 'green'
                              }
                            >
                              {record.role === 'user'
                                ? '👤 用户'
                                : record.role === 'system'
                                ? '⚙️ 系统'
                                : record.role === 'assistant'
                                ? '🤖 助手'
                                : record.role}
                            </Label>
                            <span style={{ fontSize: '12px', color: '#666' }}>
                              {timestamp2string(record.created_time)}
                            </span>
                            {isError && (
                              <Label size='tiny' color='red'>
                                错误
                              </Label>
                            )}
                            {record.id === selectedRecord.id && (
                              <Label size='tiny' color='orange'>
                                当前记录
                              </Label>
                            )}
                          </div>

                          {/* 消息内容 */}
                          <div
                            style={{
                              padding: '8px',
                              background: 'white',
                              borderRadius: '4px',
                              fontSize: '14px',
                              lineHeight: '1.6',
                              whiteSpace: 'pre-wrap',
                              wordBreak: 'break-word',
                            }}
                          >
                            {record.content || (
                              <span
                                style={{ color: '#999', fontStyle: 'italic' }}
                              >
                                无内容
                              </span>
                            )}
                          </div>

                          {/* Token信息 */}
                          {(record.prompt_tokens > 0 ||
                            record.completion_tokens > 0) && (
                            <div
                              style={{
                                marginTop: '8px',
                                fontSize: '12px',
                                color: '#666',
                                display: 'flex',
                                gap: '15px',
                              }}
                            >
                              <span>📊 Prompt: {record.prompt_tokens}</span>
                              <span>
                                📝 Completion: {record.completion_tokens}
                              </span>
                              <span>💰 Total: {record.total_tokens}</span>
                              {record.response_time > 0 && (
                                <span>⏱️ {record.response_time}ms</span>
                              )}
                            </div>
                          )}

                          {/* 错误信息 */}
                          {record.error_message && (
                            <div
                              style={{
                                marginTop: '8px',
                                padding: '8px',
                                background: '#ffebee',
                                borderRadius: '4px',
                                fontSize: '12px',
                                fontFamily: 'monospace',
                                color: '#c62828',
                                border: '1px solid #ef9a9a',
                              }}
                            >
                              ❌ {record.error_message}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          )}
        </Modal.Content>
        <Modal.Actions>
          <Button onClick={closeDetailModal}>关闭</Button>
        </Modal.Actions>
      </Modal>

      <Form onSubmit={searchChannels}>
        <Form.Input
          icon='search'
          fluid
          iconPosition='left'
          placeholder={t('channel.search')}
          value={searchKeyword}
          loading={searching}
          onChange={handleKeywordChange}
        />
      </Form>
      {showPrompt && (
        <Message
          onDismiss={() => {
            setShowPrompt(false);
            setPromptShown(promptID);
          }}
        >
          {t('channel.balance_notice')}
          <br />
          {t('channel.test_notice')}
          <br />
          {t('channel.detail_notice')}
        </Message>
      )}
      <Table basic={'very'} compact size='small'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('id');
              }}
            >
              {t('channel.table.id')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('name');
              }}
            >
              {t('channel.table.name')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('group');
              }}
            >
              {t('channel.table.group')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('type');
              }}
            >
              {t('channel.table.type')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('status');
              }}
            >
              {t('channel.table.status')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('response_time');
              }}
            >
              {t('channel.table.response_time')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('balance');
              }}
            >
              {t('channel.table.balance')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('error_count');
              }}
            >
              错误/总数
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('priority');
              }}
              hidden={!showDetail}
            >
              {t('channel.table.priority')}
            </Table.HeaderCell>
            <Table.HeaderCell hidden={!showDetail}>
              {t('channel.table.test_model')}
            </Table.HeaderCell>
            <Table.HeaderCell>{t('channel.table.actions')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {channels
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE
            )
            .map((channel, idx) => {
              if (channel.deleted) return <></>;
              return (
                <Table.Row key={channel.id}>
                  <Table.Cell>{channel.id}</Table.Cell>
                  <Table.Cell>
                    {channel.name ? channel.name : t('channel.table.no_name')}
                  </Table.Cell>
                  <Table.Cell>{renderGroup(channel.group)}</Table.Cell>
                  <Table.Cell>{renderType(channel.type, t)}</Table.Cell>
                  <Table.Cell>{renderStatus(channel.status, t)}</Table.Cell>
                  <Table.Cell>
                    <Popup
                      content={
                        channel.test_time
                          ? renderTimestamp(channel.test_time)
                          : t('channel.table.not_tested')
                      }
                      key={channel.id}
                      trigger={renderResponseTime(channel.response_time, t)}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Popup
                      trigger={
                        <span
                          onClick={() => {
                            updateChannelBalance(channel.id, channel.name, idx);
                          }}
                          style={{ cursor: 'pointer' }}
                        >
                          {renderBalance(channel.type, channel.balance, t)}
                        </span>
                      }
                      content={t('channel.table.click_to_update')}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <div
                      style={{
                        display: 'flex',
                        gap: '4px',
                        alignItems: 'center',
                      }}
                    >
                      <Popup
                        trigger={
                          <Label
                            basic
                            color={channel.error_count > 0 ? 'red' : 'grey'}
                            style={{
                              cursor:
                                channel.error_count > 0 ? 'pointer' : 'default',
                            }}
                            onClick={() => {
                              if (channel.error_count > 0) {
                                openErrorRecordsModal(channel.id, channel.name);
                              }
                            }}
                          >
                            {channel.error_count || 0}
                          </Label>
                        }
                        content={
                          channel.error_count > 0
                            ? '点击查看错误记录'
                            : '暂无错误'
                        }
                        basic
                      />
                      <span>/</span>
                      <Label basic>{channel.total_count || 0}</Label>
                    </div>
                  </Table.Cell>
                  <Table.Cell hidden={!showDetail}>
                    <Popup
                      trigger={
                        <Input
                          type='number'
                          defaultValue={channel.priority}
                          onBlur={(event) => {
                            manageChannel(
                              channel.id,
                              'priority',
                              idx,
                              event.target.value
                            );
                          }}
                        >
                          <input style={{ maxWidth: '60px' }} />
                        </Input>
                      }
                      content={t('channel.table.priority_tip')}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell hidden={!showDetail}>
                    <Dropdown
                      placeholder={t('channel.table.select_test_model')}
                      selection
                      options={channel.model_options}
                      defaultValue={channel.test_model}
                      onChange={(event, data) => {
                        switchTestModel(idx, data.value);
                      }}
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '2px',
                        rowGap: '6px',
                      }}
                    >
                      <Button
                        size={'tiny'}
                        positive
                        onClick={() => {
                          testChannel(
                            channel.id,
                            channel.name,
                            idx,
                            channel.test_model
                          );
                        }}
                      >
                        {t('channel.buttons.test')}
                      </Button>
                      <Popup
                        trigger={
                          <Button size='tiny' negative>
                            {t('channel.buttons.delete')}
                          </Button>
                        }
                        on='click'
                        flowing
                        hoverable
                      >
                        <Button
                          size={'tiny'}
                          negative
                          onClick={() => {
                            manageChannel(channel.id, 'delete', idx);
                          }}
                        >
                          {t('channel.buttons.confirm_delete')} {channel.name}
                        </Button>
                      </Popup>
                      <Button
                        size={'tiny'}
                        onClick={() => {
                          manageChannel(
                            channel.id,
                            channel.status === 1 ? 'disable' : 'enable',
                            idx
                          );
                        }}
                      >
                        {channel.status === 1
                          ? t('channel.buttons.disable')
                          : t('channel.buttons.enable')}
                      </Button>
                      <Button
                        size={'tiny'}
                        as={Link}
                        to={'/channel/edit/' + channel.id}
                      >
                        {t('channel.buttons.edit')}
                      </Button>
                    </div>
                  </Table.Cell>
                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan={showDetail ? '11' : '9'}>
              <Button size='tiny' as={Link} to='/channel/add' loading={loading}>
                {t('channel.buttons.add')}
              </Button>
              <Button
                size='tiny'
                loading={loading}
                onClick={() => {
                  testChannels('all');
                }}
              >
                {t('channel.buttons.test_all')}
              </Button>
              <Button
                size='tiny'
                loading={loading}
                onClick={() => {
                  testChannels('disabled');
                }}
              >
                {t('channel.buttons.test_disabled')}
              </Button>
              <Popup
                trigger={
                  <Button size='tiny' loading={loading}>
                    {t('channel.buttons.delete_disabled')}
                  </Button>
                }
                on='click'
                flowing
                hoverable
              >
                <Button
                  size='tiny'
                  loading={loading}
                  negative
                  onClick={deleteAllDisabledChannels}
                >
                  {t('channel.buttons.confirm_delete_disabled')}
                </Button>
              </Popup>
              <Pagination
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                size='tiny'
                siblingRange={1}
                totalPages={
                  Math.ceil(channels.length / ITEMS_PER_PAGE) +
                  (channels.length % ITEMS_PER_PAGE === 0 ? 1 : 0)
                }
              />
              <Button size='tiny' onClick={refresh} loading={loading}>
                {t('channel.buttons.refresh')}
              </Button>
              <Button size='tiny' onClick={toggleShowDetail}>
                {showDetail
                  ? t('channel.buttons.hide_detail')
                  : t('channel.buttons.show_detail')}
              </Button>
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default ChannelsTable;
