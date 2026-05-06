function renderCodex(codex) {
  const root = document.getElementById('codex-section');
  if (!root) return;
  if (!codex || !codex.available) {
    root.innerHTML = `
      <h2>Codex 网络稳定性</h2>
      <div class="panel">
        <div class="empty">${escapeHTML((codex && codex.error) || '未检测到 Codex 日志。')}</div>
      </div>
    `;
    return;
  }

  const summary = codex.summary || {};
  root.innerHTML = `
    <div class="codex-title-row">
      <h2>Codex 网络稳定性</h2>
      <span>${escapeHTML(codex.range_label || '')}${codex.clamped ? ' · 已按最近 24 小时统计' : ''}</span>
    </div>
    <div class="codex-grid">
      ${renderCodexMetric('断流重试', `${summary.retry_events || 0} / ${summary.completed_turns || 0}`, `${summary.retry_affected_turn_rate || '0%'} / 完成 turn`)}
      ${renderCodexMetric('受影响 turn', summary.retry_affected_turns || 0, `${summary.retry_affected_turn_rate || '0%'} / 完成 turn`)}
      ${renderCodexMetric('网络错误', summary.network_candidates || 0, 'timeout / DNS / TLS / 5xx')}
      ${renderCodexMetric('最大重试深度', summary.max_retry_attempt || '0/5', '自动恢复深度')}
    </div>
    <article class="card chart codex-chart">
      <div class="chart-header">
        <h3>Codex 网络异常时间轴</h3>
        <span>次数</span>
      </div>
      <svg viewBox="0 0 820 240" role="img" aria-label="Codex 网络异常时间轴"></svg>
      <div class="chart-tooltip" aria-hidden="true"></div>
    </article>
  `;
  drawCodexTimeline(root.querySelector('svg'), root.querySelector('.chart-tooltip'), codex.timeline || []);
}

function renderCodexLoading(message) {
  const root = document.getElementById('codex-section');
  if (!root) return;
  root.innerHTML = `
    <h2>Codex 网络稳定性</h2>
    <div class="panel">
      <div class="empty">${escapeHTML(message || '正在分析 Codex 本地日志...')}</div>
    </div>
  `;
}

function renderCodexMetric(label, value, hint) {
  return `
    <article class="card codex-metric">
      <div class="metric-label">${escapeHTML(label)}</div>
      <div class="metric-value">${escapeHTML(String(value))}</div>
      <div class="codex-hint">${escapeHTML(hint)}</div>
    </article>
  `;
}

function drawCodexTimeline(svg, tooltip, points) {
  const rect = svg.getBoundingClientRect();
  const width = Math.max(560, Math.round(rect.width || 820));
  const height = Math.max(220, Math.round(rect.height || 240));
  svg.setAttribute('viewBox', `0 0 ${width} ${height}`);
  svg.setAttribute('preserveAspectRatio', 'none');
  const padding = { top: 24, right: 22, bottom: 30, left: 48 };
  if (!points.length) {
    tooltip.remove();
    svg.innerHTML = `<text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="#6b7280">无数据</text>`;
    return;
  }

  const series = [
    { key: 'stream_retry', label: '断流重试', color: '#d65d47' },
    { key: 'network_candidate', label: '网络错误', color: '#2a9d8f' },
  ];
  const values = points.flatMap((point) => series.map((item) => Number(point[item.key] || 0)));
  const maxValue = Math.max(0, ...values);
  const maxY = Math.max(4, Math.ceil(maxValue / 4) * 4);
  const times = points.map((point) => new Date(point.ts).getTime());
  const minX = Math.min(...times);
  const maxX = Math.max(...times);
  const plotWidth = width - padding.left - padding.right;
  const plotHeight = height - padding.top - padding.bottom;
  const x = (value) => padding.left + ((value - minX) / Math.max(1, maxX - minX)) * plotWidth;
  const y = (value) => padding.top + (1 - (Number(value || 0) / maxY)) * plotHeight;
  const plotted = points.map((point) => {
    const cx = x(new Date(point.ts).getTime());
    const positions = {};
    series.forEach((item) => {
      positions[item.key] = {
        value: Number(point[item.key] || 0),
        cy: y(point[item.key]),
      };
    });
    return { ...point, cx, positions };
  });

  const polylines = series.map((item) => {
    const polyline = plotted.map((point) => `${point.cx.toFixed(1)},${point.positions[item.key].cy.toFixed(1)}`).join(' ');
    return `<polyline fill="none" stroke="${item.color}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" points="${polyline}" />`;
  }).join('');

  let grid = '';
  for (let index = 0; index <= 4; index += 1) {
    const gy = padding.top + (plotHeight / 4) * index;
    const tickValue = Math.round(maxY - (maxY / 4) * index);
    grid += `<line x1="${padding.left}" y1="${gy}" x2="${width - padding.right}" y2="${gy}" stroke="rgba(38,70,83,0.12)" />`;
    grid += `<text x="${padding.left - 10}" y="${gy + 4}" text-anchor="end" font-size="11" fill="#6b7280">${tickValue}</text>`;
  }
  const legend = series.map((item, index) => {
    const lx = padding.left + index * 108;
    return `<g><line x1="${lx}" y1="11" x2="${lx + 18}" y2="11" stroke="${item.color}" stroke-width="2.5" stroke-linecap="round" /><text x="${lx + 24}" y="15" font-size="11" fill="#6b7280">${item.label}</text></g>`;
  }).join('');
  svg.innerHTML = `
    ${legend}
    ${grid}
    <line x1="${padding.left}" y1="${height - padding.bottom}" x2="${width - padding.right}" y2="${height - padding.bottom}" stroke="#264653" />
    <line x1="${padding.left}" y1="${padding.top}" x2="${padding.left}" y2="${height - padding.bottom}" stroke="#264653" />
    ${polylines}
    <g class="codex-hover" visibility="hidden">
      <line class="codex-crosshair" y1="${padding.top}" y2="${height - padding.bottom}" stroke="#6b7280" stroke-dasharray="4 3" stroke-width="1.3" opacity="0.75" />
      ${series.map((item) => `<circle class="codex-active-point" data-key="${item.key}" r="4.3" fill="${item.color}" stroke="#fffdf8" stroke-width="2" />`).join('')}
    </g>
    <rect class="chart-overlay" x="${padding.left}" y="${padding.top}" width="${plotWidth}" height="${plotHeight}" fill="transparent" />
    <text x="${padding.left}" y="${height - 8}" font-size="11" fill="#6b7280">${formatCodexTime(points[0].ts)}</text>
    <text x="${width - padding.right}" y="${height - 8}" text-anchor="end" font-size="11" fill="#6b7280">${formatCodexTime(points[points.length - 1].ts)}</text>
  `;

  const hoverLayer = svg.querySelector('.codex-hover');
  const crosshair = svg.querySelector('.codex-crosshair');
  const activePoints = Array.from(svg.querySelectorAll('.codex-active-point'));
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
    activePoints.forEach((circle) => {
      const position = point.positions[circle.dataset.key];
      circle.setAttribute('cx', point.cx);
      circle.setAttribute('cy', position.cy);
    });
    tooltip.innerHTML = `
      <div class="chart-tooltip-label">时间</div>
      <div class="chart-tooltip-value">${escapeHTML(formatCodexTooltipTime(point.ts))}</div>
      ${series.map((item) => `
        <div class="chart-tooltip-label" style="margin-top:6px;">${item.label}</div>
        <div class="chart-tooltip-value">${formatCodexTooltipMetric(item.key, point)}</div>
      `).join('')}
    `;
    positionCodexTooltip(svg, chartCard, tooltip, point);
    tooltip.classList.add('is-visible');
  };

  overlay.addEventListener('mousemove', (event) => {
    const hoverX = getCodexSvgX(svg, event);
    showHoverState(getNearestCodexPoint(plotted, hoverX));
  });
  overlay.addEventListener('mouseleave', hideHoverState);
}

function formatCodexTooltipMetric(key, point) {
  const value = point.positions[key].value;
  if (key === 'stream_retry') {
    return `${value} 次 / ${Number(point.completed_turns || 0)} 轮`;
  }
  return String(value);
}

function getCodexSvgX(svg, event) {
  const point = svg.createSVGPoint();
  point.x = event.clientX;
  point.y = event.clientY;
  const matrix = svg.getScreenCTM();
  if (!matrix) return 0;
  return point.matrixTransform(matrix.inverse()).x;
}

function positionCodexTooltip(svg, chartCard, tooltip, point) {
  const svgRect = svg.getBoundingClientRect();
  const cardRect = chartCard.getBoundingClientRect();
  const viewBox = svg.viewBox.baseVal;
  const topCy = Math.min(...Object.values(point.positions).map((item) => item.cy));
  const left = svgRect.left - cardRect.left + (point.cx / viewBox.width) * svgRect.width;
  const top = svgRect.top - cardRect.top + (topCy / viewBox.height) * svgRect.height;
  const halfWidth = tooltip.offsetWidth / 2;
  tooltip.style.left = `${Math.min(Math.max(left, halfWidth + 8), chartCard.clientWidth - halfWidth - 8)}px`;
  tooltip.style.top = `${Math.max(top, tooltip.offsetHeight + 12)}px`;
}

function getNearestCodexPoint(points, hoverX) {
  return points.reduce((nearest, point) => {
    if (!nearest) return point;
    return Math.abs(point.cx - hoverX) < Math.abs(nearest.cx - hoverX) ? point : nearest;
  }, null);
}

function formatCodexTime(value) {
  const date = new Date(value);
  return `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

function formatCodexTooltipTime(value) {
  const date = new Date(value);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  }[char]));
}
