import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import * as d3 from 'd3';
import { JSDOM } from 'jsdom';

const here = dirname(fileURLToPath(import.meta.url));
const C = {
  bg: '#F7F8FA', paper: '#FFFFFF', ink: '#1E2026', muted: '#61666F',
  line: '#DFE2E7', red: '#EF3124', redTint: '#FDECEB',
  blue: '#2C58B5', blueTint: '#E8EEFA', green: '#0F7451',
  greenTint: '#E4F3EC', amber: '#A35700', amberTint: '#FFF1DC',
  navy: '#202C42', navyLight: '#34435D', white: '#FFFFFF',
};
const FONT = 'Arial, Helvetica, sans-serif';

function canvas(width, height, title, description) {
  const document = new JSDOM('<!doctype html><html><body></body></html>').window.document;
  const svg = d3.select(document.body).append('svg')
    .attr('xmlns', 'http://www.w3.org/2000/svg')
    .attr('viewBox', `0 0 ${width} ${height}`)
    .attr('width', width).attr('height', height)
    .attr('role', 'img').attr('aria-labelledby', 'diagram-title diagram-description');
  svg.append('title').attr('id', 'diagram-title').text(title);
  svg.append('desc').attr('id', 'diagram-description').text(description);
  const defs = svg.append('defs');
  const filter = defs.append('filter').attr('id', 'card-shadow')
    .attr('x', '-10%').attr('y', '-10%').attr('width', '120%').attr('height', '135%');
  filter.append('feDropShadow').attr('dx', 0).attr('dy', 7).attr('stdDeviation', 10)
    .attr('flood-color', '#1E2026').attr('flood-opacity', 0.06);
  for (const [name, color] of [['red', C.red], ['blue', C.blue], ['gray', '#AAB4C2']]) {
    defs.append('marker').attr('id', `arrow-${name}`)
      .attr('viewBox', '0 0 12 12').attr('refX', 11).attr('refY', 6)
      .attr('markerWidth', 8).attr('markerHeight', 8).attr('orient', 'auto')
      .append('path').attr('d', 'M 1 1 L 11 6 L 1 11 Z').attr('fill', color);
  }
  svg.append('rect').attr('width', width).attr('height', height).attr('fill', C.bg);
  return { document, svg };
}

function rect(svg, x, y, w, h, fill, stroke = 'none', radius = 20, shadow = false) {
  return svg.append('rect').attr('x', x).attr('y', y).attr('width', w).attr('height', h)
    .attr('rx', radius).attr('fill', fill).attr('stroke', stroke)
    .attr('stroke-width', stroke === 'none' ? 0 : 1.3)
    .attr('filter', shadow ? 'url(#card-shadow)' : null);
}

function text(svg, value, x, y, size, color = C.ink, weight = 400, options = {}) {
  return svg.append('text').attr('x', x).attr('y', y).attr('fill', color)
    .attr('font-family', FONT).attr('font-size', size).attr('font-weight', weight)
    .attr('letter-spacing', options.spacing || null)
    .attr('text-anchor', options.anchor || 'start').text(value);
}

function lines(svg, values, x, y, size = 17, color = C.muted, lineHeight = 25, weight = 400) {
  values.forEach((value, index) => text(svg, value, x, y + index * lineHeight, size, color, weight));
}

function pill(svg, label, x, y, w, h, fill, color, fontSize = 14, stroke = 'none') {
  rect(svg, x, y, w, h, fill, stroke, h / 2);
  text(svg, label, x + w / 2, y + h / 2 + fontSize * 0.34, fontSize, color, 700, { anchor: 'middle' });
}

function line(svg, x1, y1, x2, y2, color = '#AAB4C2', width = 2.5, marker = null) {
  svg.append('line').attr('x1', x1).attr('y1', y1).attr('x2', x2).attr('y2', y2)
    .attr('stroke', color).attr('stroke-width', width).attr('stroke-linecap', 'round')
    .attr('marker-end', marker ? `url(#arrow-${marker})` : null);
}

function header(svg, eyebrow, title, subtitle, width = 1280) {
  rect(svg, 40, 43, 7, 72, C.red, 'none', 3);
  text(svg, eyebrow, 68, 62, 13, C.red, 800, { spacing: 2.2 });
  text(svg, title, 68, 100, 36, C.ink, 800);
  text(svg, subtitle, 68, 130, 16, C.muted, 400);
  pill(svg, 'SMART CREDIT  /  MVP', width - 261, 52, 220, 33, C.white, C.muted, 12, C.line);
}

function dbIcon(svg, x, y, color) {
  svg.append('ellipse').attr('cx', x + 10).attr('cy', y + 4).attr('rx', 10).attr('ry', 4)
    .attr('fill', 'none').attr('stroke', color).attr('stroke-width', 1.4);
  svg.append('path').attr('d', `M${x},${y + 4} v12 c0,5 20,5 20,0 v-12`)
    .attr('fill', 'none').attr('stroke', color).attr('stroke-width', 1.4);
}

function architecture() {
  const { document, svg } = canvas(1280, 850, 'Архитектура Smart Credit',
    'Клиентский и банковский сервисы обмениваются событиями через Kafka; у каждого своя база.');
  header(svg, 'АРХИТЕКТУРА', 'Два контура. Один поток событий.',
    'Карта MVP: интерфейсы, сервисы, хранение и локальные проверки.');

  rect(svg, 40, 160, 1200, 142, C.white, C.line, 22, true);
  text(svg, 'ТОЧКИ ВХОДА  ·  HTTP', 66, 191, 12, C.muted, 800, { spacing: 1.7 });
  pill(svg, 'Клиент', 66, 215, 116, 50, C.redTint, C.red, 16);
  pill(svg, 'Аналитик', 194, 215, 132, 50, C.redTint, C.red, 16);
  line(svg, 339, 240, 375, 240, '#AAB4C2', 2.2, 'gray');
  rect(svg, 391, 209, 215, 62, C.bg, C.line, 14);
  text(svg, 'web', 413, 236, 18, C.ink, 700);
  text(svg, 'кабинет · backoffice', 413, 257, 13, C.muted);
  rect(svg, 622, 209, 240, 62, C.bg, C.line, 14);
  text(svg, 'gateway + Swagger', 644, 236, 17, C.ink, 700);
  text(svg, 'единый адрес /api и /docs', 644, 257, 13, C.muted);
  rect(svg, 878, 209, 334, 62, C.bg, C.line, 14);
  text(svg, 'auth', 900, 236, 18, C.ink, 700);
  text(svg, 'JWT · роли', 900, 257, 13, C.muted);
  dbIcon(svg, 1081, 229, C.muted);
  text(svg, 'auth_db', 1112, 245, 13, C.muted, 700);

  text(svg, 'БИЗНЕС-ОБМЕН  ·  ТОЛЬКО KAFKA', 43, 350, 12, C.muted, 800, { spacing: 1.7 });
  const top = 374, height = 225;
  rect(svg, 40, top, 350, height, C.white, C.line, 24, true);
  rect(svg, 40, top, 350, 7, C.blue, 'none', 3);
  text(svg, 'КЛИЕНТСКИЙ КОНТУР', 66, top + 36, 12, C.blue, 800, { spacing: 1.4 });
  text(svg, 'client-service', 66, top + 77, 28, C.ink, 800);
  lines(svg, ['Профиль, заявки, документы', 'и публичная проекция'], 66, top + 109, 16, C.muted, 24);
  rect(svg, 66, top + 159, 296, 43, C.blueTint, 'none', 12);
  dbIcon(svg, 84, top + 170, C.blue);
  text(svg, 'client_db', 117, top + 186, 15, C.blue, 700);
  text(svg, 'PostgreSQL', 270, top + 186, 13, C.blue, 400);

  rect(svg, 465, top, 350, height, C.navy, 'none', 24, true);
  rect(svg, 465, top, 350, 7, C.red, 'none', 3);
  text(svg, 'АСИНХРОННАЯ ГРАНИЦА', 491, top + 36, 12, '#BFC9DB', 800, { spacing: 1.4 });
  text(svg, 'Kafka', 491, top + 77, 30, C.white, 800);
  lines(svg, ['События и публичные снимки', 'без общей БД'], 491, top + 109, 16, '#D5DEED', 24);
  rect(svg, 491, top + 153, 298, 54, C.navyLight, 'none', 12);
  svg.append('circle').attr('cx', 512).attr('cy', top + 171).attr('r', 5).attr('fill', C.red);
  text(svg, 'client.events  →  банк', 528, top + 176, 14, C.white, 700);
  svg.append('circle').attr('cx', 512).attr('cy', top + 192).attr('r', 5).attr('fill', '#8AA9FF');
  text(svg, 'bank.events  →  клиент', 528, top + 197, 14, C.white, 700);

  rect(svg, 890, top, 350, height, C.white, C.line, 24, true);
  rect(svg, 890, top, 350, 7, C.green, 'none', 3);
  text(svg, 'БАНКОВСКИЙ КОНТУР', 916, top + 36, 12, C.green, 800, { spacing: 1.4 });
  text(svg, 'bank-service', 916, top + 77, 28, C.ink, 800);
  lines(svg, ['Дело, правила, оферта', 'и аудит решения'], 916, top + 109, 16, C.muted, 24);
  rect(svg, 916, top + 159, 296, 43, C.greenTint, 'none', 12);
  dbIcon(svg, 934, top + 170, C.green);
  text(svg, 'bank_db', 967, top + 186, 15, C.green, 700);
  text(svg, 'PostgreSQL', 1120, top + 186, 13, C.green, 400);

  line(svg, 399, 448, 454, 448, C.red, 3, 'red');
  line(svg, 455, 528, 400, 528, C.blue, 3, 'blue');
  line(svg, 825, 448, 880, 448, C.red, 3, 'red');
  line(svg, 880, 528, 825, 528, C.blue, 3, 'blue');

  line(svg, 1065, 606, 1065, 636, C.green, 2.5, 'gray');
  rect(svg, 40, 649, 1200, 161, C.greenTint, 'none', 22);
  text(svg, 'ЛОКАЛЬНЫЕ ЗАВИСИМОСТИ БАНКА  ·  gRPC', 66, 682, 12, C.green, 800, { spacing: 1.6 });
  const deps = [
    { x: 66, title: 'bureau-mocks', detail: 'Синтетические проверки' },
    { x: 464, title: 'scoring', detail: 'Оценка риска' },
    { x: 862, title: 'assistant', detail: 'Тексты и резюме' },
  ];
  for (const dep of deps) {
    rect(svg, dep.x, 702, 352, 81, C.white, '#C9E5D8', 16);
    rect(svg, dep.x + 18, 721, 6, 42, C.green, 'none', 3);
    text(svg, dep.title, dep.x + 40, 735, 18, C.ink, 800);
    text(svg, dep.detail, dep.x + 40, 758, 14, C.muted);
  }
  text(svg, 'Техническое исключение: оригинал документа банк читает защищённым HTTP GET; состояние передаётся через Kafka.',
    43, 834, 12, C.muted, 400);
  return serialize(document);
}

function stepCard(svg, { x, y, number, role, title, body, badge, color, tint }) {
  rect(svg, x, y, 370, 232, C.white, C.line, 22, true);
  rect(svg, x, y, 370, 6, color, 'none', 3);
  pill(svg, role, x + 25, y + 23, role.length > 15 ? 160 : 125, 30, tint, color, 12);
  text(svg, String(number).padStart(2, '0'), x + 337, y + 52, 31, '#CAD0D9', 800, { anchor: 'end' });
  text(svg, title, x + 25, y + 95, 25, C.ink, 800);
  lines(svg, body, x + 25, y + 126, 16, C.muted, 24);
  line(svg, x + 25, y + 174, x + 345, y + 174, C.line, 1);
  text(svg, badge, x + 25, y + 206, 13, color, 700);
}

function journey() {
  const { document, svg } = canvas(1280, 850, 'Путь заявки Smart Credit',
    'Шесть этапов от подачи заявки до демо-выдачи: отправка, проверка, оферта, принятие, подпись, симуляция выдачи.');
  header(svg, 'ПУТЬ КЛИЕНТА', 'От заявки до демо-выдачи.',
    'Шесть шагов; между клиентским и банковским контурами идут события Kafka.');

  text(svg, '01—03  /  ДО ОФЕРТЫ', 42, 174, 12, C.muted, 800, { spacing: 1.7 });
  const xs = [40, 455, 870];
  stepCard(svg, { x: xs[0], y: 190, number: 1, role: 'КЛИЕНТ', title: 'Подача',
    body: ['Компания, сумма, срок, документы.', 'Отправка запускает рассмотрение.'],
    badge: 'client.events  ·  ApplicationSubmitted', color: C.blue, tint: C.blueTint });
  stepCard(svg, { x: xs[1], y: 190, number: 2, role: 'БАНК', title: 'Проверка',
    body: ['Бюро и scoring дают данные.', 'DECISION ждёт банковских правил.'],
    badge: 'VALIDATION  →  SCORING  →  DECISION', color: C.amber, tint: C.amberTint });
  stepCard(svg, { x: xs[2], y: 190, number: 3, role: 'БАНК → КЛИЕНТ', title: 'Оферта',
    body: ['Банк публикует условия.', 'Клиент видит активную оферту.'],
    badge: 'bank.events  →  OFFER_AVAILABLE', color: C.green, tint: C.greenTint });
  line(svg, 418, 304, 447, 304, '#AAB4C2', 2.8, 'gray');
  line(svg, 833, 304, 862, 304, '#AAB4C2', 2.8, 'gray');

  text(svg, '04—06  /  ПОСЛЕ ОФЕРТЫ', 42, 478, 12, C.muted, 800, { spacing: 1.7 });
  stepCard(svg, { x: xs[0], y: 494, number: 4, role: 'КЛИЕНТ', title: 'Принятие',
    body: ['Клиент принимает оферту.', 'Банк сверяет её offer_id.'],
    badge: 'OfferAccepted  →  FORMALIZING', color: C.blue, tint: C.blueTint });
  stepCard(svg, { x: xs[1], y: 494, number: 5, role: 'КЛИЕНТ', title: 'Демо-подпись',
    body: ['Клиент подтверждает документ.', 'Банк получает событие подписи.'],
    badge: 'client.events  ·  ContractSigned', color: C.blue, tint: C.blueTint });
  stepCard(svg, { x: xs[2], y: 494, number: 6, role: 'БАНК', title: 'Демо-выдача',
    body: ['Worker записывает результат.', 'Клиент видит итоговый статус.'],
    badge: 'bank.events  ·  DISBURSED', color: C.green, tint: C.greenTint });
  line(svg, 418, 608, 447, 608, '#AAB4C2', 2.8, 'gray');
  line(svg, 833, 608, 862, 608, '#AAB4C2', 2.8, 'gray');

  rect(svg, 40, 759, 1200, 53, C.redTint, 'none', 15);
  rect(svg, 59, 773, 5, 25, C.red, 'none', 2);
  text(svg, 'DISBURSED — запись результата симуляции, а не денежный перевод.',
    80, 792, 15, C.ink, 600);
  return serialize(document);
}

function serialize(document) {
  return '<?xml version="1.0" encoding="UTF-8"?>\n' + document.querySelector('svg').outerHTML + '\n';
}

writeFileSync(join(here, 'architecture.svg'), architecture());
writeFileSync(join(here, 'client-journey.svg'), journey());
