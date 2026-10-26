import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import * as d3 from 'd3';
import { JSDOM } from 'jsdom';

const here = dirname(fileURLToPath(import.meta.url));
const C = {
  bg: '#F5F7FA', paper: '#FFFFFF', ink: '#172234', muted: '#506174',
  line: '#CFD8E3', red: '#D92F26', redTint: '#FFF0EE',
  blue: '#2055A2', blueTint: '#EAF2FF', green: '#18704D',
  greenTint: '#E7F5EF', amber: '#9B5900', amberTint: '#FFF3DF',
  navy: '#1C304D', slate: '#EAF0F6', white: '#FFFFFF',
};
const FONT = 'Arial, Helvetica, sans-serif';
function canvas(width, height, title, description) {
  const document = new JSDOM('<!doctype html><html><body></body></html>').window.document;
  const svg = d3.select(document.body).append('svg').attr('xmlns', 'http://www.w3.org/2000/svg')
    .attr('viewBox', `0 0 ${width} ${height}`).attr('width', width).attr('height', height)
    .attr('role', 'img').attr('aria-labelledby', 'diagram-title diagram-description');
  svg.append('title').attr('id', 'diagram-title').text(title);
  svg.append('desc').attr('id', 'diagram-description').text(description);
  const defs = svg.append('defs');
  defs.append('filter').attr('id', 'shadow').attr('x', '-10%').attr('y', '-10%')
    .attr('width', '120%').attr('height', '135%').append('feDropShadow')
    .attr('dx', 0).attr('dy', 5).attr('stdDeviation', 8).attr('flood-color', C.ink).attr('flood-opacity', .07);
  for (const [name, color] of [['gray', C.muted], ['blue', C.blue], ['green', C.green], ['red', C.red], ['amber', C.amber]]) {
    defs.append('marker').attr('id', `arrow-${name}`).attr('viewBox', '0 0 12 12')
      .attr('refX', 11).attr('refY', 6).attr('markerWidth', 8).attr('markerHeight', 8)
      .attr('orient', 'auto').append('path').attr('d', 'M 1 1 L 11 6 L 1 11 Z').attr('fill', color);
  }
  svg.append('rect').attr('width', width).attr('height', height).attr('fill', C.bg);
  return { document, svg };
}
function rect(svg, x, y, w, h, fill, stroke='none', radius=18, shadow=false) {
  return svg.append('rect').attr('x',x).attr('y',y).attr('width',w).attr('height',h)
    .attr('rx',radius).attr('fill',fill).attr('stroke',stroke).attr('stroke-width',stroke==='none'?0:1.4)
    .attr('filter',shadow?'url(#shadow)':null);
}
function text(svg, value, x, y, size=18, color=C.ink, weight=400, anchor='start') {
  return svg.append('text').attr('x',x).attr('y',y).attr('fill',color).attr('font-family',FONT)
    .attr('font-size',size).attr('font-weight',weight).attr('text-anchor',anchor).text(value);
}
function lines(svg, values, x, y, size=17, color=C.muted, lineHeight=25, weight=400) {
  values.forEach((v,i)=>text(svg,v,x,y+i*lineHeight,size,color,weight));
}
function pill(svg,label,x,y,w,h,fill,color,size=15) {
  rect(svg,x,y,w,h,fill,'none',h/2);
  text(svg,label,x+w/2,y+h/2+size*.34,size,color,700,'middle');
}
function arrow(svg,x1,y1,x2,y2,color=C.muted, marker='gray',width=2.5) {
  svg.append('line').attr('x1',x1).attr('y1',y1).attr('x2',x2).attr('y2',y2)
    .attr('stroke',color).attr('stroke-width',width).attr('stroke-linecap','round')
    .attr('marker-end',`url(#arrow-${marker})`);
}
function header(svg,kicker,title,subtitle,w) {
  rect(svg,46,43,7,76,C.red,'none',3);
  text(svg,kicker,74,64,14,C.red,800);
  text(svg,title,74,104,37,C.ink,800);
  text(svg,subtitle,74,136,17,C.muted);
  pill(svg,'SMART CREDIT  /  MVP',w-286,53,240,34,C.paper,C.muted,13);
}
function serialize(document){return '<?xml version="1.0" encoding="UTF-8"?>\n'+document.querySelector('svg').outerHTML+'\n';}
function component(svg,x,y,w,title,subtitle,tone=C.blue,tint=C.blueTint) {
  rect(svg,x,y,w,92,C.paper,C.line,17,true); rect(svg,x,y,6,92,tone,'none',3);
  text(svg,title,x+22,y+36,21,C.ink,800); text(svg,subtitle,x+22,y+66,15,C.muted);
}
function db(svg,x,y,w,name,detail,tone) {
  rect(svg,x,y,w,72,C.paper,C.line,14);
  svg.append('ellipse').attr('cx',x+28).attr('cy',y+28).attr('rx',11).attr('ry',5).attr('fill','none').attr('stroke',tone).attr('stroke-width',2);
  svg.append('path').attr('d',`M${x+17},${y+28}v17c0,7 22,7 22,0v-17`).attr('fill','none').attr('stroke',tone).attr('stroke-width',2);
  text(svg,name,x+53,y+32,17,C.ink,800); text(svg,detail,x+53,y+54,13,C.muted);
}
function relation(svg,x,y,w,from,to,protocol,detail,tone=C.blue,tint=C.blueTint) {
  rect(svg,x,y,w,54,C.paper,C.line,11);
  text(svg,from,x+16,y+22,15,C.ink,700);
  text(svg,'→',x+206,y+22,18,tone,800);
  text(svg,to,x+236,y+22,15,C.ink,700);
  pill(svg,protocol,x+w-146,y+10,130,24,tint,tone,12);
  text(svg,detail,x+16,y+44,13,C.muted);
}
function architecture() {
  const {document,svg}=canvas(1500,1850,'Архитектура Smart Credit','Карта сервисов, трёх независимых баз, Kafka и всех протоколов между компонентами.');
  header(svg,'АРХИТЕКТУРА','Карта MVP и соединений','Сверху — компоненты и их данные; ниже — направленные каналы с протоколами.',1500);
  text(svg,'01  ТОЧКИ ВХОДА',52,189,14,C.muted,800);
  component(svg,52,207,335,'Браузер','клиент · аналитик · Swagger',C.red,C.redTint);
  component(svg,416,207,335,'web / nginx','кабинет и backoffice',C.red,C.redTint);
  component(svg,780,207,335,'gateway / nginx','API и /docs',C.red,C.redTint);
  component(svg,1144,207,304,'Swagger UI','просмотр OpenAPI',C.red,C.redTint);
  text(svg,'02  СЕРВИСЫ И ИХ ХРАНИЛИЩА',52,354,14,C.muted,800);
  component(svg,52,373,420,'auth','JWT · refresh · JWKS',C.blue,C.blueTint);
  component(svg,540,373,420,'client-service','заявки · публичное состояние',C.blue,C.blueTint);
  component(svg,1028,373,420,'bank-service','дела · правила · решения',C.green,C.greenTint);
  arrow(svg,262,469,262,491,C.blue,'blue'); arrow(svg,750,469,750,491,C.blue,'blue'); arrow(svg,1238,469,1238,491,C.green,'green');
  db(svg,52,502,420,'auth_db','PostgreSQL · auth',C.blue);
  db(svg,540,502,420,'client_db','PostgreSQL · client-service',C.blue);
  db(svg,1028,502,420,'bank_db','PostgreSQL · bank-service',C.green);
  text(svg,'03  ШИНА СОБЫТИЙ И ЛОКАЛЬНЫЕ ЗАВИСИМОСТИ',52,635,14,C.muted,800);
  rect(svg,52,653,1396,83,C.navy,'none',16);
  text(svg,'Kafka',76,687,23,C.white,800); text(svg,'client.events   →   банк',310,686,17,C.white,700);
  text(svg,'bank.events   →   клиент',780,686,17,C.white,700);
  text(svg,'protobuf',1300,687,15,'#D8E4F3',700);
  component(svg,52,767,335,'bureau-mocks','синтетические проверки',C.green,C.greenTint);
  component(svg,416,767,335,'scoring','оценка риска',C.green,C.greenTint);
  component(svg,780,767,335,'assistant','тексты · шаблон',C.green,C.greenTint);
  component(svg,1144,767,304,'Ollama (опция)','модель на хосте',C.amber,C.amberTint);
  text(svg,'04  КАНАЛЫ МЕЖДУ КОМПОНЕНТАМИ',52,920,14,C.muted,800);
  text(svg,'Каждая строка задаёт направление. Цвет и подпись обозначают протокол; обмен состоянием клиента и банка — только через Kafka.',52,950,15,C.muted);
  const left=[
    ['Браузер','web','HTTP','Кабинет клиента и backoffice'],
    ['Браузер','gateway','HTTP','Swagger /docs и API'],
    ['web','auth','HTTP','/api/auth/*'],
    ['web','client-service','HTTP','/api/client/*'],
    ['web','bank-service','HTTP','/api/backoffice/*'],
    ['gateway','auth','HTTP','/api/auth/*'],
    ['gateway','client-service','HTTP','/api/client/*'],
    ['gateway','bank-service','HTTP','/api/backoffice/*'],
    ['gateway','Swagger UI','HTTP','Спецификации /docs'],
    ['client-service','auth','HTTP','JWKS для проверки JWT'],
    ['bank-service','auth','HTTP','JWKS для проверки JWT'],
  ];
  const right=[
    ['auth','auth_db','PostgreSQL','Учётки и refresh-сессии'],
    ['client-service','client_db','PostgreSQL','Заявки и outbox'],
    ['bank-service','bank_db','PostgreSQL','Дела, аудит и outbox'],
    ['client-service','Kafka','Kafka','Пишет client.events; читает bank.events'],
    ['bank-service','Kafka','Kafka','Читает client.events; пишет bank.events'],
    ['bank-service','client-service','HTTP','Защищённый GET оригинала документа'],
    ['bank-service','bureau-mocks','gRPC','Проверки компании и истории'],
    ['bank-service','scoring','gRPC','Score и причины'],
    ['bank-service','assistant','gRPC','Резюме и объяснение'],
    ['client-service','assistant','gRPC','Help по оценке'],
    ['assistant','Ollama','HTTP','Опциональная локальная модель'],
  ];
  const tint=(p)=>p==='Kafka'?[C.red,C.redTint]:p==='gRPC'?[C.green,C.greenTint]:p==='PostgreSQL'?[C.amber,C.amberTint]:[C.blue,C.blueTint];
  left.forEach((r,i)=>relation(svg,52,976+i*66,680,...r,...tint(r[2])));
  right.forEach((r,i)=>relation(svg,768,976+i*66,680,...r,...tint(r[2])));
  rect(svg,52,1730,1396,72,C.slate,'none',14);
  text(svg,'Данные: один контейнер PostgreSQL, три отдельные БД. Документы клиента и RSA-ключ auth — отдельные Docker volumes.',76,1773,17,C.ink,600);
  return serialize(document);
}
function flowCard(svg,x,y,w,h,eyebrow,title,body,tone=C.blue,tint=C.blueTint) {
  rect(svg,x,y,w,h,C.paper,C.line,18,true);rect(svg,x,y,7,h,tone,'none',3);
  pill(svg,eyebrow,x+22,y+20,Math.min(w-44,Math.max(134,eyebrow.length*9+24)),29,tint,tone,12);
  text(svg,title,x+22,y+82,25,C.ink,800);
  lines(svg,body,x+22,y+114,16,C.muted,24);
}
function branch(svg,x,y,w,title,body,tone=C.amber,tint=C.amberTint) {
  const h=72+body.length*24;
  rect(svg,x,y,w,h,tint,'none',14);
  rect(svg,x+16,y+18,5,h-36,tone,'none',3);
  text(svg,title,x+34,y+37,19,tone,800);
  lines(svg,body,x+34,y+65,15,C.ink,23);
  return h;
}
function spine(svg,ys,x,w,color=C.blue) {
  for(let i=0;i<ys.length-1;i++)arrow(svg,x+w/2,ys[i]+190,x+w/2,ys[i+1]-15,color,color===C.green?'green':'blue',3);
}
function flow(type) {
  const cfg={
    client:{h:2120,title:'Путь клиента',subtitle:'Публичные статусы и действия от черновика до демо-выдачи.',steps:[
      ['01 · КЛИЕНТ','DRAFT',['Профиль компании, продукт, форма и документы.','/submit → ApplicationSubmitted в client.events.']],
      ['02 · ОЖИДАНИЕ БАНКА','SUBMITTED',['Заявка отправлена; банк создаёт дело.','Ответ на submit ещё не является решением.']],
      ['03 · ПРОВЕРКА','UNDER_REVIEW',['VALIDATION → ENRICHMENT → SCORING → DECISION.','Публичное состояние приходит через bank.events.']],
      ['04 · ПРЕДЛОЖЕНИЕ','OFFER_AVAILABLE',['Получить условия и срок оферты.','/offer/accept → ACCEPT_PENDING.']],
      ['05 · ОФОРМЛЕНИЕ','FORMALIZING',['Банк подтвердил OfferAccepted.','/sign → SIGN_PENDING → ContractSigned.']],
      ['06 · ИТОГ','DISBURSED',['Банк обработал подпись и опубликовал итог.','Денежный перевод не выполняется.']],
    ],branches:[
      [0,'Ошибки заполнения',['422 INVALID_APPLICATION → остаётся DRAFT.']],
      [2,'Банк просит данные',['NEEDS_INFO → /info → InfoProvided → новая проверка.']],
      [2,'Банк отказывает',['REJECTED → публичная причина в карточке.']],
      [2,'Ручной разбор',['Для клиента остаётся UNDER_REVIEW; решает аналитик.']],
      [2,'Оценка SME_CASHFLOW_V2',['Варианты CALCULATED не являются офертой.','Условия подтверждает аналитик банка.']],
      [3,'Отказ или срок',['/offer/decline → DECLINE_PENDING → CLOSED.','Истечение оферты у банка → CLOSED.']],
      [4,'Отзыв до подписи',['/cancel → CANCEL_PENDING → CLOSED.']],
    ],color:C.blue,tint:C.blueTint},
    bank:{h:2640,title:'Путь банковского дела и аналитика',subtitle:'Внутренние статусы, автоматические правила и действия сотрудника.',steps:[
      ['01 · СОБЫТИЕ','VALIDATION',['ApplicationSubmitted из client.events.','Банк создаёт дело и проверяет полноту.']],
      ['02 · ПРОВЕРКИ','ENRICHMENT',['gRPC bureau-mocks: компания и кредитная история.','Результат сохраняется в bank_db.']],
      ['03 · ОЦЕНКА','SCORING',['gRPC scoring: score, grade и причины.','Результат не является кредитным решением.']],
      ['04 · ПРАВИЛА','DECISION',['Worker применяет версию правил продукта.','Это ожидание исхода, не одобрение.']],
      ['05 · УСЛОВИЯ','OFFER / COUNTER_OFFER',['Банк публикует оферту в bank.events.','Клиент видит OFFER_AVAILABLE.']],
      ['06 · ПРИНЯТИЕ','OFFER_ACCEPTED',['OfferAccepted из client.events; сверяется offer_id.','Клиенту доступна демо-подпись.']],
      ['07 · ПОДПИСЬ','SIGNING',['ContractSigned из client.events.','Worker выполняет демо-выдачу.']],
      ['08 · ИТОГ','DISBURSED',['Банк публикует финальный снимок.','Фактических денег система не переводит.']],
    ],branches:[
      [0,'Неизвестная версия продукта',['MANUAL_REVIEW: дело доступно аналитику.']],
      [0,'Недостающие данные',['NEEDS_INFO → запрос клиенту.','InfoProvided → обратно в VALIDATION.']],
      [3,'Автоматические исходы',['OFFER · NEEDS_INFO · MANUAL_REVIEW · REJECTED.']],
      [3,'MANUAL_REVIEW → решение аналитика',['/bank: список → карточка → /take.','/decision с ETag и Idempotency-Key.','APPROVE / COUNTER → оферта; REQUEST_INFO → NEEDS_INFO;','REJECT → REJECTED.']],
      [3,'Путь оценки v2',['SME_CASHFLOW_V2: варианты CALCULATED.','Аналитик подтверждает актуальный вариант.']],
      [3,'Отказ банка',['REJECTED → публичная причина для клиента.']],
      [4,'Оферта не принята',['OfferDeclined или отзыв → CANCELLED.','Срок expires_at → EXPIRED.']],
    ],color:C.green,tint:C.greenTint},
    auth:{h:1740,title:'Вход и сессия',subtitle:'Один механизм для клиента, аналитика и администратора.',steps:[
      ['01 · ЛОГИН','POST /auth/login',['Email и пароль демонстрационной учётки.','auth сверяет хеш пароля в auth_db.']],
      ['02 · ДОСТУП','access JWT + refresh-cookie',['RS256 JWT передаётся как Bearer.','HttpOnly cookie хранит refresh-сессию.']],
      ['03 · ЗАЩИЩЁННЫЕ API','GET /auth/me',['client-service и bank-service читают JWKS auth.','Роль JWT ограничивает доступ к своим API.']],
      ['04 · ПРОДЛЕНИЕ','POST /auth/refresh',['По действующей cookie выдаётся новый access JWT.','Срок самой cookie не продлевается.']],
      ['05 · ВЫХОД','POST /auth/logout',['Refresh-сессия отзывается; cookie очищается.','Выданный access живёт до своего exp.']],
    ],branches:[
      [0,'Неверные данные',['Неизвестный пользователь или неверный пароль → 401.']],
      [2,'Неверный доступ',['Нет Bearer JWT / не та роль → 401 / 403.']],
      [3,'Refresh недействителен',['Новый access не выдаётся; нужен вход.']],
    ],color:C.blue,tint:C.blueTint},
  }[type];
  const {document,svg}=canvas(1480,cfg.h,`Smart Credit — ${cfg.title}`,cfg.subtitle);
  header(svg,'СЦЕНАРИЙ СЕРВИСА',cfg.title,cfg.subtitle,1480);
  const x=58,w=665,ry=194,gap=310;
  const ys=cfg.steps.map((_,i)=>ry+i*gap);
  spine(svg,ys,x,w,cfg.color);
  cfg.steps.forEach((s,i)=>flowCard(svg,x,ys[i],w,190,s[0],s[1],s[2],cfg.color,cfg.tint));
  let nextBranchY=ry;
  cfg.branches.forEach(([stage,title,body])=>{
    const by=Math.max(ys[stage],nextBranchY);
    const h=branch(svg,825,by,600,title,body);
    const cy=by+Math.min(65,h/2);
    svg.append('path').attr('d',`M ${x+w+8} ${ys[stage]+95} H 770 V ${cy} H 810`)
      .attr('fill','none').attr('stroke',C.amber).attr('stroke-width',2.1)
      .attr('stroke-linecap','round').attr('stroke-linejoin','round')
      .attr('marker-end','url(#arrow-amber)');
    nextBranchY=by+h+18;
  });
  return serialize(document);
}
writeFileSync(join(here,'architecture.svg'),architecture());
writeFileSync(join(here,'client-flow.svg'),flow('client'));
writeFileSync(join(here,'bank-flow.svg'),flow('bank'));
writeFileSync(join(here,'auth-flow.svg'),flow('auth'));
