# Dashboard Frontend Refactor Plan

## Goal

Improve the Dashboard page so it reads as an operational usage dashboard instead of a set of isolated charts. The changes should keep the existing API stable unless a later step explicitly needs backend data.

## Current Problems

- The top three cards mix metric title and total value in one line, which becomes crowded.
- The main numbers do not have a clear visual hierarchy.
- The lower chart is labeled too generally as "统计".
- Chart axis numbers are too raw for large token values.
- There is only one analysis view, so quota, token, and request count are not easy to compare separately.

## Refactor Steps

### Step 1: Top Metric Cards

Status: Completed

- Change the top cards to use this structure:
  - title
  - primary total number
  - "最近 7 天" context label
  - compact trend line chart
- Keep the existing three metrics:
  - request total
  - quota total
  - token total
- Keep the existing `/api/user/dashboard` API and existing data calculation.

### Step 2: Main Analysis Section

Status: Completed

- Rename the lower section from "统计" to "模型数据分析".
- Add a secondary subtitle showing the current time range.
- Format large axis values, especially token counts.

### Step 3: Analysis Tabs

Status: Completed

- Add chart tabs:
  - Token 分布
  - 额度分布
  - 请求次数
- Reuse the existing dashboard data first.
- Avoid backend API changes unless model-level quota and request ranking need more precise data.

### Step 4: Model Ranking

Status: Completed

- Add model ranking cards or a compact table:
  - model name
  - requests
  - quota
  - tokens
- Use consistent number formatting.

### Step 5: Polish

Status: Completed

- Reduce one-note purple usage.
- Make chart colors map consistently to metric types.
- Improve mobile layout for long totals.
- Keep card radius, spacing, and typography consistent with the rest of the project.

## Verification

- Run `npm run build` in `web/default`.
- Start the Go server again because frontend assets are embedded by `go:embed`.
- Use local test rows in `one-api.db` to confirm totals and chart data.
