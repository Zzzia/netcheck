const { liveMode, initialPayload, defaultRange } = window.reportConfig;
let currentQuery = null;
let codexRequestSeq = 0;
let codexLastQueryKey = '';
let codexLastFetchAt = 0;
const codexRefreshIntervalMs = 120000;

document.querySelectorAll('[data-range]').forEach((button) => {
  button.addEventListener('click', () => {
    if (!liveMode) return;
    currentQuery = { range: button.dataset.range };
    setActivePreset(button.dataset.range);
    loadRange(currentQuery, false);
  });
});

document.getElementById('apply-custom').addEventListener('click', () => {
  if (!liveMode) return;
  const start = document.getElementById('start-input').value;
  const end = document.getElementById('end-input').value;
  if (!start || !end) {
    window.alert('请同时填写开始和结束时间');
    return;
  }
  currentQuery = { start, end };
  clearPresetActive();
  loadRange(currentQuery, false);
});

function bootstrap() {
  if (liveMode) {
    currentQuery = { range: defaultRange };
    setActivePreset(defaultRange);
    loadRange(currentQuery, false);
    window.setInterval(() => loadRange(currentQuery, true), 30000);
    return;
  }
  disableToolbar();
  render(initialPayload, false);
}

async function loadRange(params, preserveControls) {
  try {
    const query = new URLSearchParams(params);
    const response = await fetch(`/api/report-data?${query.toString()}`);
    if (!response.ok) {
      throw new Error(await response.text());
    }
    const payload = await response.json();
    render(payload, preserveControls);
    if (liveMode && shouldLoadCodex(params, preserveControls)) {
      loadCodex(params, !preserveControls);
    }
  } catch (error) {
    document.getElementById('generated-at').innerHTML = `<span class="error">${error.message}</span>`;
  }
}

function shouldLoadCodex(params, preserveControls) {
  const queryKey = new URLSearchParams(params).toString();
  if (!preserveControls || queryKey !== codexLastQueryKey) {
    return true;
  }
  return Date.now() - codexLastFetchAt > codexRefreshIntervalMs;
}

async function loadCodex(params, showLoading) {
  const requestID = ++codexRequestSeq;
  codexLastQueryKey = new URLSearchParams(params).toString();
  codexLastFetchAt = Date.now();
  if (showLoading) {
    renderCodexLoading('正在分析 Codex 本地日志...');
  }
  try {
    const query = new URLSearchParams(params);
    const response = await fetch(`/api/codex-data?${query.toString()}`);
    if (!response.ok) {
      throw new Error(await response.text());
    }
    const payload = await response.json();
    if (requestID !== codexRequestSeq) return;
    renderCodex(payload);
  } catch (error) {
    if (requestID !== codexRequestSeq) return;
    renderCodex({ available: false, error: error.message });
  }
}

function render(payload, preserveControls) {
  document.getElementById('generated-at').textContent = `更新时间：${payload.generated_at}`;
  renderSummary(payload.summary || []);
  renderEventMeta(payload.event_meta || { count: 0, longest: '0s' });
  renderCauses(payload.causes || []);
  renderGroups(payload.groups || []);
  if (!liveMode) {
    renderCodex(payload.codex || null);
  }
  renderEvents(payload.events || []);
  if (liveMode && !preserveControls) {
    document.getElementById('start-input').value = toDateTimeLocal(payload.range_start);
    document.getElementById('end-input').value = toDateTimeLocal(payload.range_end);
  }
}

function renderSummary(cards) {
  const root = document.getElementById('summary-grid');
  root.innerHTML = '';
  cards.forEach((card) => {
    const article = document.createElement('article');
    article.className = 'card';
    article.innerHTML = `
      <h2>${card.title}</h2>
      <div class="metric-list" style="--metric-columns:${Math.max((card.metrics || []).length, 1)};">
        ${(card.metrics || []).map((metric) => renderSummaryMetric(metric)).join('')}
      </div>
    `;
    root.appendChild(article);
  });
}

function renderSummaryMetric(metric) {
  if (metric.lines && metric.lines.length) {
    return `
      <div class="metric-entry">
        <div class="metric-label">${metric.label}</div>
        <div class="metric-stack">
          ${metric.lines.map((line) => `
            <div class="metric-row">
              <span class="metric-value">${line.value}</span>
              <span class="metric-tag">${line.label}</span>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  }
  return `
    <div class="metric-entry">
      <div class="metric-label">${metric.label}</div>
      <div class="metric-row">
        <span class="metric-value">${metric.value}</span>
      </div>
    </div>
  `;
}

function renderEventMeta(meta) {
  document.getElementById('event-meta').innerHTML = `
    <div class="metric-label">异常次数</div>
    <div class="metric-value">${meta.count}</div>
    <div class="metric-label" style="margin-top:12px;">最长持续</div>
    <div class="metric-value">${meta.longest}</div>
  `;
}

function renderCauses(causes) {
  const root = document.getElementById('cause-table');
  if (!causes.length) {
    root.innerHTML = '<div class="empty">当前时间范围内没有异常归因数据。</div>';
    return;
  }
  root.innerHTML = `
    <table>
      <thead><tr><th>类别</th><th>累计时长</th></tr></thead>
      <tbody>
        ${causes.map((item) => `<tr><td>${item.name}</td><td>${item.duration}</td></tr>`).join('')}
      </tbody>
    </table>
  `;
}

function renderGroups(groups) {
  const root = document.getElementById('group-container');
  root.innerHTML = '';
  groups.forEach((group) => {
    const section = document.createElement('section');
    section.className = 'dimension';
    section.innerHTML = `<h2>${group.title}</h2><div class="dimension-grid"></div>`;
    root.appendChild(section);
    const grid = section.querySelector('.dimension-grid');
    (group.charts || []).forEach((chart) => {
      const article = document.createElement('article');
      article.className = 'card chart';
      article.innerHTML = `
        <div class="chart-header">
          <h3>${chart.title}</h3>
          <span>${chart.unit}</span>
        </div>
        <svg viewBox="0 0 560 220" role="img" aria-label="${chart.title}"></svg>
        <div class="chart-tooltip" aria-hidden="true"></div>
      `;
      grid.appendChild(article);
      drawChart(article.querySelector('svg'), article.querySelector('.chart-tooltip'), chart);
    });
  });
}

function renderEvents(events) {
  const root = document.getElementById('event-table');
  if (!events.length) {
    root.innerHTML = '<div class="empty">当前时间范围内没有事件记录。</div>';
    return;
  }
  root.innerHTML = `
    <table>
      <thead>
        <tr>
          <th>类别</th>
          <th>状态</th>
          <th>摘要</th>
          <th>证据</th>
          <th>开始</th>
          <th>结束</th>
          <th>持续</th>
        </tr>
      </thead>
      <tbody>
        ${events.map((item) => `
          <tr>
            <td>${item.name}</td>
            <td>${item.status}</td>
            <td>${item.summary}</td>
            <td>${item.evidence}</td>
            <td>${item.started_at}</td>
            <td>${item.ended_at}</td>
            <td>${item.duration}</td>
          </tr>
        `).join('')}
      </tbody>
    </table>
  `;
}

function drawChart(svg, tooltip, chart) {
  const width = 560;
  const height = 220;
  const padding = { top: 12, right: 16, bottom: 26, left: 48 };
  const series = chart.series[0];
  const points = series.points || [];
  if (!points.length) {
    tooltip.remove();
    svg.innerHTML = `<text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="#6b7280">无数据</text>`;
    return;
  }

  const values = points.map((item) => item.value);
  const times = points.map((item) => new Date(item.ts).getTime());
  const minX = Math.min(...times);
  const maxX = Math.max(...times);
  let minY = chart.start_at_zero ? 0 : Math.min(...values);
  let maxY = Math.max(...values);
  if (chart.unit === '比例') {
    maxY = maxY === 0 ? 0.1 : Math.min(1, Math.max(maxY * 1.25, 0.05));
  }
  if (minY === maxY) {
    maxY = maxY === 0 ? 1 : maxY * 1.2;
  }
  const plotWidth = width - padding.left - padding.right;
  const plotHeight = height - padding.top - padding.bottom;
  const x = (value) => padding.left + ((value - minX) / Math.max(1, maxX - minX)) * plotWidth;
  const y = (value) => padding.top + (1 - ((value - minY) / Math.max(1, maxY - minY))) * plotHeight;
  const plotPoints = points.map((point) => ({
    ...point,
    cx: x(new Date(point.ts).getTime()),
    cy: y(point.value),
  }));
  const polyline = plotPoints.map((point) => `${point.cx.toFixed(1)},${point.cy.toFixed(1)}`).join(' ');

  const ticks = 4;
  let grid = '';
  for (let index = 0; index <= ticks; index += 1) {
    const gy = padding.top + (plotHeight / ticks) * index;
    const tickValue = maxY - ((maxY - minY) / ticks) * index;
    grid += `<line x1="${padding.left}" y1="${gy}" x2="${width - padding.right}" y2="${gy}" stroke="rgba(38,70,83,0.12)" />`;
    grid += `<text x="${padding.left - 10}" y="${gy + 4}" text-anchor="end" font-size="11" fill="#6b7280">${tickValue.toFixed(chart.unit === '比例' ? 2 : 1)}</text>`;
  }

  const startLabel = formatTime(points[0].ts, chart.time_format);
  const endLabel = formatTime(points[points.length - 1].ts, chart.time_format);
  svg.innerHTML = `
    ${grid}
    <line x1="${padding.left}" y1="${height - padding.bottom}" x2="${width - padding.right}" y2="${height - padding.bottom}" stroke="#264653" />
    <line x1="${padding.left}" y1="${padding.top}" x2="${padding.left}" y2="${height - padding.bottom}" stroke="#264653" />
    <polyline fill="none" stroke="${series.color}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" points="${polyline}" />
    <g class="chart-hover" visibility="hidden">
      <line class="chart-crosshair" y1="${padding.top}" y2="${height - padding.bottom}" stroke="${series.color}" stroke-dasharray="4 3" stroke-width="1.5" opacity="0.7" />
      <circle class="chart-active-point" r="4.5" fill="${series.color}" stroke="#fffdf8" stroke-width="2" />
    </g>
    <rect class="chart-overlay" x="${padding.left}" y="${padding.top}" width="${plotWidth}" height="${plotHeight}" fill="transparent" />
    <text x="${padding.left}" y="${height - 8}" font-size="11" fill="#6b7280">${startLabel}</text>
    <text x="${width - padding.right}" y="${height - 8}" text-anchor="end" font-size="11" fill="#6b7280">${endLabel}</text>
  `;

  const hoverLayer = svg.querySelector('.chart-hover');
  const crosshair = svg.querySelector('.chart-crosshair');
  const activePoint = svg.querySelector('.chart-active-point');
  const overlay = svg.querySelector('.chart-overlay');
  const chartCard = tooltip.parentElement;
  const hideHoverState = () => {
    hoverLayer.setAttribute('visibility', 'hidden');
    tooltip.classList.remove('is-visible');
  };
  const showHoverState = (point) => {
    hoverLayer.setAttribute('visibility', 'visible');
    crosshair.setAttribute('x1', point.cx);
    crosshair.setAttribute('x2', point.cx);
    activePoint.setAttribute('cx', point.cx);
    activePoint.setAttribute('cy', point.cy);
    tooltip.innerHTML = `
      <div class="chart-tooltip-label">时间</div>
      <div class="chart-tooltip-value">${formatTooltipTime(point.ts, chart.time_format)}</div>
      <div class="chart-tooltip-label" style="margin-top:6px;">数值</div>
      <div class="chart-tooltip-value">${formatTooltipValue(point.value, chart.unit)}</div>
    `;
    positionTooltip(svg, chartCard, tooltip, point);
    tooltip.classList.add('is-visible');
  };

  overlay.addEventListener('mousemove', (event) => {
    const svgRect = svg.getBoundingClientRect();
    const scaleX = width / svgRect.width;
    const hoverX = (event.clientX - svgRect.left) * scaleX;
    const nearest = getNearestPoint(plotPoints, hoverX);
    showHoverState(nearest);
  });
  overlay.addEventListener('mouseleave', hideHoverState);
}

function getNearestPoint(points, hoverX) {
  return points.reduce((nearest, point) => {
    if (!nearest) {
      return point;
    }
    return Math.abs(point.cx - hoverX) < Math.abs(nearest.cx - hoverX) ? point : nearest;
  }, null);
}

function positionTooltip(svg, chartCard, tooltip, point) {
  const svgRect = svg.getBoundingClientRect();
  const cardRect = chartCard.getBoundingClientRect();
  const viewBox = svg.viewBox.baseVal;
  const left = svgRect.left - cardRect.left + (point.cx / viewBox.width) * svgRect.width;
  const top = svgRect.top - cardRect.top + (point.cy / viewBox.height) * svgRect.height;
  const halfWidth = tooltip.offsetWidth / 2;
  const clampedLeft = Math.min(Math.max(left, halfWidth + 8), chartCard.clientWidth - halfWidth - 8);
  const clampedTop = Math.max(top, tooltip.offsetHeight + 12);
  tooltip.style.left = `${clampedLeft}px`;
  tooltip.style.top = `${clampedTop}px`;
}

function formatTime(value, mode) {
  const date = new Date(value);
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hour = String(date.getHours()).padStart(2, '0');
  const minute = String(date.getMinutes()).padStart(2, '0');
  const second = String(date.getSeconds()).padStart(2, '0');
  if (mode === 'second') return `${hour}:${minute}:${second}`;
  if (mode === 'hour') return `${month}-${day} ${hour}:00`;
  return `${hour}:${minute}`;
}

function formatTooltipTime(value, mode) {
  const date = new Date(value);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hour = String(date.getHours()).padStart(2, '0');
  const minute = String(date.getMinutes()).padStart(2, '0');
  const second = String(date.getSeconds()).padStart(2, '0');
  if (mode === 'second') return `${year}-${month}-${day} ${hour}:${minute}:${second}`;
  if (mode === 'hour') return `${year}-${month}-${day} ${hour}:00`;
  return `${year}-${month}-${day} ${hour}:${minute}`;
}

function formatTooltipValue(value, unit) {
  if (unit === '比例') {
    return `${value.toFixed(2)} (${(value * 100).toFixed(1)}%)`;
  }
  return `${value.toFixed(2)} ${unit}`;
}

function setActivePreset(range) {
  document.querySelectorAll('[data-range]').forEach((button) => {
    button.classList.toggle('active', button.dataset.range === range);
  });
}

function clearPresetActive() {
  document.querySelectorAll('[data-range]').forEach((button) => button.classList.remove('active'));
}

function disableToolbar() {
  document.querySelectorAll('.toolbar button, .toolbar input').forEach((node) => {
    node.disabled = true;
  });
}

function toDateTimeLocal(value) {
  return value.replace(' ', 'T');
}

bootstrap();
