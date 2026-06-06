import React, {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Card, Grid} from 'semantic-ui-react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import axios from 'axios';
import './Dashboard.css';

// 在 Dashboard 组件内添加自定义配置
const chartConfig = {
  lineChart: {
    style: {
      background: '#fff',
      borderRadius: '8px',
    },
    line: {
      strokeWidth: 2,
      dot: false,
      activeDot: { r: 4 },
    },
    grid: {
      vertical: false,
      horizontal: true,
      opacity: 0.1,
    },
  },
  colors: {
    requests: '#2563EB',
    quota: '#F59E0B',
    tokens: '#EC4899',
  },
  barColors: [
    '#33527A',
    '#14B8A6',
    '#F59E0B',
    '#EC4899',
    '#22C55E',
    '#6366F1',
    '#0EA5E9',
    '#A855F7',
    '#F97316',
    '#64748B',
  ],
};

const analysisTabs = [
  {
    key: 'tokens',
    label: 'Token 分布',
    title: '模型 Token 分布',
    totalLabel: '总 Tokens',
  },
  {
    key: 'quota',
    label: '额度分布',
    title: '模型额度分布',
    totalLabel: '总额度',
  },
  {
    key: 'requests',
    label: '请求次数',
    title: '模型请求次数',
    totalLabel: '总请求',
  },
];

const getQuotaPerUnit = () => {
  const quotaPerUnit = parseFloat(
    localStorage.getItem('quota_per_unit') || '500000'
  );
  return Number.isFinite(quotaPerUnit) && quotaPerUnit > 0
    ? quotaPerUnit
    : 500000;
};

const formatCurrency = (value) => `$${Number(value || 0).toFixed(2)}`;

const formatTokenCount = (value) =>
  Math.round(Number(value || 0)).toLocaleString('en-US');

const formatCompactNumber = (value) => {
  const number = Number(value || 0);
  if (Math.abs(number) >= 1000000000) {
    return `${(number / 1000000000).toFixed(1)}B`;
  }
  if (Math.abs(number) >= 1000000) {
    return `${(number / 1000000).toFixed(1)}M`;
  }
  if (Math.abs(number) >= 1000) {
    return `${(number / 1000).toFixed(1)}K`;
  }
  return Math.round(number).toString();
};

const Dashboard = () => {
  const { t } = useTranslation();
  const [data, setData] = useState([]);
  const [activeAnalysis, setActiveAnalysis] = useState('tokens');
  const [summaryData, setSummaryData] = useState({
    totalRequests: 0,
    totalQuota: 0,
    totalTokens: 0,
  });

  useEffect(() => {
    fetchDashboardData();
  }, []);

  const fetchDashboardData = async () => {
    try {
      const response = await axios.get('/api/user/dashboard');
      if (response.data.success) {
        const dashboardData = response.data.data || [];
        setData(dashboardData);
        calculateSummary(dashboardData);
      }
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error);
      setData([]);
      calculateSummary([]);
    }
  };

  const calculateSummary = (dashboardData) => {
    if (!Array.isArray(dashboardData) || dashboardData.length === 0) {
      setSummaryData({
        totalRequests: 0,
        totalQuota: 0,
        totalTokens: 0,
      });
      return;
    }

    const summary = {
      totalRequests: dashboardData.reduce(
        (sum, item) => sum + item.RequestCount,
        0
      ),
      totalQuota:
        dashboardData.reduce((sum, item) => sum + item.Quota, 0) /
        getQuotaPerUnit(),
      totalTokens: dashboardData.reduce(
        (sum, item) => sum + item.PromptTokens + item.CompletionTokens,
        0
      ),
    };

    setSummaryData(summary);
  };

  // 处理数据以供折线图使用，补充缺失的日期
  const processTimeSeriesData = () => {
    const dailyData = {};

    // 获取日期范围
    const dates = data.map((item) => item.Day);
    const maxDate = new Date(); // 总是使用今天作为最后一天
    let minDate =
      dates.length > 0
        ? new Date(Math.min(...dates.map((d) => new Date(d))))
        : new Date();

    // 确保至少显示7天的数据
    const sevenDaysAgo = new Date();
    sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 6); // -6是因为包含今天
    if (minDate > sevenDaysAgo) {
      minDate = sevenDaysAgo;
    }

    // 生成所有日期
    for (let d = new Date(minDate); d <= maxDate; d.setDate(d.getDate() + 1)) {
      const dateStr = d.toISOString().split('T')[0];
      dailyData[dateStr] = {
        date: dateStr,
        requests: 0,
        quota: 0,
        tokens: 0,
      };
    }

    // 填充实际数据
    data.forEach((item) => {
      dailyData[item.Day].requests += item.RequestCount;
      dailyData[item.Day].quota += item.Quota / getQuotaPerUnit();
      dailyData[item.Day].tokens += item.PromptTokens + item.CompletionTokens;
    });

    return Object.values(dailyData).sort((a, b) =>
      a.date.localeCompare(b.date)
    );
  };

  const getMetricValue = (item, metricKey) => {
    if (metricKey === 'quota') {
      return item.Quota / getQuotaPerUnit();
    }
    if (metricKey === 'requests') {
      return item.RequestCount;
    }
    return item.PromptTokens + item.CompletionTokens;
  };

  const formatAnalysisValue = (value, metricKey = activeAnalysis) => {
    if (metricKey === 'quota') {
      return formatCurrency(value);
    }
    return formatTokenCount(value);
  };

  const formatAnalysisAxisValue = (value) => {
    if (activeAnalysis === 'quota') {
      return `$${formatCompactNumber(value).replace(/\.0([KMB])$/, '$1')}`;
    }
    return formatCompactNumber(value);
  };

  // 处理数据以供堆叠柱状图使用
  const processModelData = (metricKey) => {
    const timeData = {};

    // 获取日期范围
    const dates = data.map((item) => item.Day);
    const maxDate = new Date(); // 总是使用今天作为最后一天
    let minDate =
      dates.length > 0
        ? new Date(Math.min(...dates.map((d) => new Date(d))))
        : new Date();

    // 确保至少显示7天的数据
    const sevenDaysAgo = new Date();
    sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 6); // -6是因为包含今天
    if (minDate > sevenDaysAgo) {
      minDate = sevenDaysAgo;
    }

    // 生成所有日期
    for (let d = new Date(minDate); d <= maxDate; d.setDate(d.getDate() + 1)) {
      const dateStr = d.toISOString().split('T')[0];
      timeData[dateStr] = {
        date: dateStr,
      };

      // 初始化所有模型的数据为0
      const models = [...new Set(data.map((item) => item.ModelName))];
      models.forEach((model) => {
        timeData[dateStr][model] = 0;
      });
    }

    // 填充实际数据
    data.forEach((item) => {
      timeData[item.Day][item.ModelName] += getMetricValue(item, metricKey);
    });

    return Object.values(timeData).sort((a, b) => a.date.localeCompare(b.date));
  };

  const processModelRankingData = () => {
    const modelStats = {};
    data.forEach((item) => {
      const modelName = item.ModelName || 'unknown';
      if (!modelStats[modelName]) {
        modelStats[modelName] = {
          model: modelName,
          requests: 0,
          quota: 0,
          tokens: 0,
        };
      }
      modelStats[modelName].requests += item.RequestCount;
      modelStats[modelName].quota += item.Quota / getQuotaPerUnit();
      modelStats[modelName].tokens += item.PromptTokens + item.CompletionTokens;
    });
    return Object.values(modelStats).sort(
      (a, b) => b[activeAnalysis] - a[activeAnalysis]
    );
  };

  // 获取所有唯一的模型名称
  const getUniqueModels = () => {
    return [...new Set(data.map((item) => item.ModelName))];
  };

  const timeSeriesData = processTimeSeriesData();
  const modelData = processModelData(activeAnalysis);
  const modelRankingData = processModelRankingData();
  const models = getUniqueModels();
  const activeAnalysisConfig =
    analysisTabs.find((item) => item.key === activeAnalysis) || analysisTabs[0];
  const activeTotalValue =
    activeAnalysis === 'quota'
      ? summaryData.totalQuota
      : activeAnalysis === 'requests'
      ? summaryData.totalRequests
      : summaryData.totalTokens;

  // 生成随机颜色
  const getRandomColor = (index) => {
    return chartConfig.barColors[index % chartConfig.barColors.length];
  };

  // 添加一个日期格式化函数
  const formatDate = (dateStr) => {
    const date = new Date(dateStr);
    return date.toLocaleDateString('zh-CN', {
      month: 'numeric',
      day: 'numeric',
    });
  };

  // 修改所有 XAxis 配置
  const xAxisConfig = {
    dataKey: 'date',
    axisLine: false,
    tickLine: false,
    tick: {
      fontSize: 12,
      fill: '#A3AED0',
      textAnchor: 'middle', // 文本居中对齐
    },
    tickFormatter: formatDate,
    interval: 0,
    minTickGap: 5,
    padding: { left: 30, right: 30 }, // 增加两侧的内边距，确保首尾标签完整显示
  };

  const renderMetricCard = ({
    title,
    value,
    dataKey,
    color,
    tooltipLabel,
    formatter,
  }) => (
    <Card fluid className='chart-card metric-card'>
      <Card.Content>
        <div className='metric-card-header'>
          <div>
            <div className='metric-card-title'>{title}</div>
            <div className='metric-card-value'>{value}</div>
            <div className='metric-card-subtitle'>最近 7 天</div>
          </div>
        </div>
        <div className='metric-chart-container'>
          <ResponsiveContainer width='100%' height={120}>
            <LineChart data={timeSeriesData}>
              <CartesianGrid
                strokeDasharray='3 3'
                vertical={chartConfig.lineChart.grid.vertical}
                horizontal={chartConfig.lineChart.grid.horizontal}
                opacity={chartConfig.lineChart.grid.opacity}
              />
              <XAxis {...xAxisConfig} />
              <YAxis hide={true} />
              <Tooltip
                contentStyle={{
                  background: '#fff',
                  border: 'none',
                  borderRadius: '4px',
                  boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                }}
                formatter={(tooltipValue) => [
                  formatter(tooltipValue),
                  tooltipLabel,
                ]}
                labelFormatter={(label) =>
                  `${t('dashboard.statistics.tooltip.date')}: ${formatDate(
                    label
                  )}`
                }
              />
              <Line
                type='monotone'
                dataKey={dataKey}
                stroke={color}
                strokeWidth={chartConfig.lineChart.line.strokeWidth}
                dot={chartConfig.lineChart.line.dot}
                activeDot={chartConfig.lineChart.line.activeDot}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </Card.Content>
    </Card>
  );

  return (
    <div className='dashboard-container'>
      <Grid columns={3} stackable className='charts-grid'>
        <Grid.Column>
          {renderMetricCard({
            title: t('dashboard.charts.requests.title'),
            value: formatTokenCount(summaryData.totalRequests),
            dataKey: 'requests',
            color: chartConfig.colors.requests,
            tooltipLabel: t('dashboard.charts.requests.tooltip'),
            formatter: formatTokenCount,
          })}
        </Grid.Column>

        <Grid.Column>
          {renderMetricCard({
            title: t('dashboard.charts.quota.title'),
            value: formatCurrency(summaryData.totalQuota),
            dataKey: 'quota',
            color: chartConfig.colors.quota,
            tooltipLabel: t('dashboard.charts.quota.tooltip'),
            formatter: formatCurrency,
          })}
        </Grid.Column>

        <Grid.Column>
          {renderMetricCard({
            title: t('dashboard.charts.tokens.title'),
            value: formatTokenCount(summaryData.totalTokens),
            dataKey: 'tokens',
            color: chartConfig.colors.tokens,
            tooltipLabel: t('dashboard.charts.tokens.tooltip'),
            formatter: formatTokenCount,
          })}
        </Grid.Column>
      </Grid>

      <Card fluid className='chart-card analysis-card'>
        <Card.Content>
          <div className='analysis-header'>
            <div>
              <div className='analysis-title'>模型数据分析</div>
              <div className='analysis-subtitle'>最近 7 天 · 按模型聚合</div>
            </div>
            <div className='analysis-tabs'>
              {analysisTabs.map((tab) => (
                <button
                  key={tab.key}
                  type='button'
                  className={`analysis-tab${
                    activeAnalysis === tab.key ? ' active' : ''
                  }`}
                  onClick={() => setActiveAnalysis(tab.key)}
                >
                  {tab.label}
                </button>
              ))}
            </div>
          </div>

          <div className='analysis-body'>
            <div className='analysis-chart-panel'>
              <div className='analysis-chart-heading'>
                <div>
                  <div className='analysis-chart-title'>
                    {activeAnalysisConfig.title}
                  </div>
                  <div className='analysis-chart-total'>
                    {activeAnalysisConfig.totalLabel}:{' '}
                    {formatAnalysisValue(activeTotalValue)}
                  </div>
                </div>
              </div>
              <div className='chart-container analysis-chart-container'>
                <ResponsiveContainer width='100%' height={320}>
                  <BarChart data={modelData}>
                    <CartesianGrid
                      strokeDasharray='3 3'
                      vertical={false}
                      opacity={0.12}
                    />
                    <XAxis {...xAxisConfig} />
                    <YAxis
                      axisLine={false}
                      tickLine={false}
                      tick={{ fontSize: 12, fill: '#8F9BB3' }}
                      tickFormatter={formatAnalysisAxisValue}
                    />
                    <Tooltip
                      contentStyle={{
                        background: '#fff',
                        border: 'none',
                        borderRadius: '4px',
                        boxShadow: '0 2px 8px rgba(0,0,0,0.1)',
                      }}
                      labelFormatter={(label) =>
                        `${t(
                          'dashboard.statistics.tooltip.date'
                        )}: ${formatDate(label)}`
                      }
                      formatter={(value, name) => [
                        formatAnalysisValue(value),
                        name,
                      ]}
                    />
                    <Legend
                      wrapperStyle={{
                        paddingTop: '20px',
                      }}
                    />
                    {models.map((model, index) => (
                      <Bar
                        key={model}
                        dataKey={model}
                        stackId='a'
                        fill={getRandomColor(index)}
                        name={model}
                        radius={[4, 4, 0, 0]}
                      />
                    ))}
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </div>

            <div className='model-ranking-panel'>
              <div className='model-ranking-title'>模型排行</div>
              <div className='model-ranking-subtitle'>
                按 {activeAnalysisConfig.label} 排序
              </div>
              <div className='model-ranking-list'>
                {modelRankingData.length === 0 ? (
                  <div className='model-ranking-empty'>暂无数据</div>
                ) : (
                  modelRankingData.map((item, index) => (
                    <div className='model-ranking-row' key={item.model}>
                      <div className='model-ranking-index'>{index + 1}</div>
                      <div className='model-ranking-content'>
                        <div className='model-ranking-name'>{item.model}</div>
                        <div className='model-ranking-meta'>
                          请求 {formatTokenCount(item.requests)} · 额度{' '}
                          {formatCurrency(item.quota)} · Tokens{' '}
                          {formatTokenCount(item.tokens)}
                        </div>
                      </div>
                      <div className='model-ranking-value'>
                        {formatAnalysisValue(item[activeAnalysis])}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </Card.Content>
      </Card>
    </div>
  );
};

export default Dashboard;
