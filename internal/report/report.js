const { liveMode, initialPayload, defaultRange, initialLang, translations } = window.reportConfig;
let currentQuery = null;
let codexRequestSeq = 0;
let currentLang = initialLang || 'en';
let lastPayload = null;
let lastCodexPayload = null;

document.querySelectorAll('[data-lang]').forEach((button) => {
  button.addEventListener('click', () => {
    currentLang = button.dataset.lang || 'en';
    document.documentElement.lang = currentLang;
    renderStaticLabels();
    updateLanguageButtons();
    if (lastPayload) {
      render(lastPayload, true);
    }
    if (lastCodexPayload) {
      renderCodex(lastCodexPayload);
    }
    if (liveMode && currentQuery) {
      loadRange(currentQuery, true);
    }
  });
});

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
    window.alert(t('ui.alert.custom_range'));
    return;
  }
  currentQuery = { start, end };
  clearPresetActive();
  loadRange(currentQuery, false);
});

function bootstrap() {
  renderStaticLabels();
  updateLanguageButtons();
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
    const query = withLang(params);
    const response = await fetch(`/api/report-data?${query.toString()}`);
    if (!response.ok) {
      throw new Error(await response.text());
    }
    const payload = await response.json();
    render(payload, preserveControls);
    if (liveMode) {
      loadCodex(params, !preserveControls);
    }
  } catch (error) {
    document.getElementById('generated-at').innerHTML = `<span class="error">${error.message}</span>`;
  }
}

async function loadCodex(params, showLoading) {
  const requestID = ++codexRequestSeq;
  if (showLoading) {
    renderCodexLoading(t('codex.loading'));
  }
  try {
    const query = withLang(params);
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
  lastPayload = payload;
  document.getElementById('generated-at').textContent = formatText('ui.updated_at', payload.generated_at);
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
      <h2>${translateValue(card.title_key, card.title)}</h2>
      <div class="metric-list">
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
        <div class="metric-label">${translateValue(metric.label_key, metric.label)}</div>
        <div class="metric-stack">
          ${metric.lines.map((line) => `
            <div class="metric-row">
              <span class="metric-value">${line.value}</span>
              <span class="metric-tag">${translateValue(line.label_key, line.label)}</span>
            </div>
          `).join('')}
        </div>
      </div>
    `;
  }
  return `
    <div class="metric-entry">
      <div class="metric-label">${translateValue(metric.label_key, metric.label)}</div>
      <div class="metric-row">
        <span class="metric-value">${metric.value}</span>
      </div>
    </div>
  `;
}

function renderEventMeta(meta) {
  document.getElementById('event-meta').innerHTML = `
    <div class="metric-label">${t('ui.incident_count')}</div>
    <div class="metric-value">${meta.count}</div>
    <div class="metric-label" style="margin-top:12px;">${t('ui.longest_duration')}</div>
    <div class="metric-value">${meta.longest}</div>
  `;
}

function renderCauses(causes) {
  const root = document.getElementById('cause-table');
  if (!causes.length) {
    root.innerHTML = `<div class="empty">${t('ui.empty_causes')}</div>`;
    return;
  }
  root.innerHTML = `
    <table>
      <thead><tr><th>${t('ui.table.category')}</th><th>${t('ui.table.total_duration')}</th></tr></thead>
      <tbody>
        ${causes.map((item) => `<tr><td>${translateValue(item.name_key, item.name)}</td><td>${item.duration}</td></tr>`).join('')}
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
    section.innerHTML = `<h2>${translateValue(group.title_key, group.title)}</h2><div class="dimension-grid"></div>`;
    root.appendChild(section);
    const grid = section.querySelector('.dimension-grid');
    (group.charts || []).forEach((chart) => {
      const article = document.createElement('article');
      article.className = 'card chart';
      article.innerHTML = `
        <div class="chart-header">
          <h3>${translateValue(chart.title_key, chart.title)}</h3>
          <span>${translateValue(chart.unit_key, chart.unit)}</span>
        </div>
        <svg viewBox="0 0 560 220" role="img" aria-label="${translateValue(chart.title_key, chart.title)}"></svg>
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
    root.innerHTML = `<div class="empty">${t('ui.empty_events')}</div>`;
    return;
  }
  root.innerHTML = `
    <table>
      <thead>
        <tr>
          <th>${t('ui.table.category')}</th>
          <th>${t('ui.table.status')}</th>
          <th>${t('ui.table.summary')}</th>
          <th>${t('ui.table.evidence')}</th>
          <th>${t('ui.table.window_start')}</th>
          <th>${t('ui.table.window_end')}</th>
          <th>${t('ui.table.window_duration')}</th>
        </tr>
      </thead>
      <tbody>
        ${events.map((item) => `
          <tr>
            <td>${translateValue(item.name_key, item.name)}</td>
            <td>${translateValue(item.status_key, item.status)}</td>
            <td>${translateLocalizedText(item.summary_i18n, item.summary)}</td>
            <td>${translateLocalizedText(item.evidence_i18n, item.evidence)}</td>
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
    svg.innerHTML = `<text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" fill="#6b7280">${t('common.no_data')}</text>`;
    return;
  }

  const values = points.map((item) => item.value);
  const times = points.map((item) => new Date(item.ts).getTime());
  const minX = Math.min(...times);
  const maxX = Math.max(...times);
  let minY = chart.start_at_zero ? 0 : Math.min(...values);
  let maxY = Math.max(...values);
  if (isRatioUnit(chart.unit, chart.unit_key)) {
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
    grid += `<text x="${padding.left - 10}" y="${gy + 4}" text-anchor="end" font-size="11" fill="#6b7280">${tickValue.toFixed(isRatioUnit(chart.unit, chart.unit_key) ? 2 : 1)}</text>`;
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
      <div class="chart-tooltip-label">${t('ui.tooltip.time')}</div>
      <div class="chart-tooltip-value">${formatTooltipTime(point.ts, chart.time_format)}</div>
      <div class="chart-tooltip-label" style="margin-top:6px;">${t('ui.tooltip.value')}</div>
      <div class="chart-tooltip-value">${formatTooltipValue(point.value, translateValue(chart.unit_key, chart.unit), chart.unit_key)}</div>
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

function formatTooltipValue(value, unit, unitKey) {
  if (isRatioUnit(unit, unitKey)) {
    return `${value.toFixed(2)} (${(value * 100).toFixed(1)}%)`;
  }
  return `${value.toFixed(2)} ${unit}`;
}

function isRatioUnit(unit, unitKey) {
  return unitKey === 'unit.ratio' || unit === 'ratio' || unit === '比例';
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

function renderStaticLabels() {
  document.querySelectorAll('[data-i18n]').forEach((node) => {
    node.textContent = t(node.dataset.i18n);
  });
  document.title = t('ui.title');
}

function updateLanguageButtons() {
  document.querySelectorAll('[data-lang]').forEach((button) => {
    button.classList.toggle('active', button.dataset.lang === currentLang);
  });
}

function withLang(params) {
  const query = new URLSearchParams(params);
  query.set('lang', currentLang);
  return query;
}

function translateValue(key, fallback) {
  return key ? t(key) : fallback;
}

function translateLocalizedText(values, fallback) {
  return (values && (values[currentLang] || values.en)) || fallback;
}

function t(key) {
  return (translations && translations[currentLang] && translations[currentLang][key]) ||
    (translations && translations.en && translations.en[key]) ||
    key;
}

function formatText(key, ...args) {
  let template = t(key);
  args.forEach((value) => {
    template = template.replace(/%[sd]/, String(value));
  });
  return template;
}

bootstrap();
