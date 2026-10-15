'use strict';

const D = JSON.parse(document.getElementById('data').textContent);
const $ = id => document.getElementById(id);
const esc = s => String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);
const icon = name => `<svg class="i" viewBox="0 0 24 24" aria-hidden="true">${ICONS[name]}</svg>`;
const clock = s => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}`;
// span is a time interval; an interval without an end goes on until the end of the battle.
const span = (a, b) => b ? tr('timeRange', clock(a), clock(b)) : tr('since', clock(a));
const num = v => v == null ? 0 : Math.round(v * 10) / 10;
const words = s => s.replace(/_/g, ' ');
const capital = s => s.charAt(0).toUpperCase() + s.slice(1);
const END = D.meta.end || D.meta.duration;
const team = n => D.teams.find(t => t.number === n);
const armyName = t => t.armyName || capital(words(t.army || ''));
const teamName = n => team(n) && team(n).army ? tr('teamArmy', n, armyName(team(n))) : tr('team', n);
const playerName = i => i >= 0 ? D.players[i].name : tr('ai');
// Names come from the game when erpl-mapgen found them, otherwise from ids.
const vehicleName = v => v.name || capital(words(v.model.replace(/^(ussr|germ|ger|us|usa|uk|jp|jap|ita|it|fr|cn)_/, '')));
const structureName = s => s.name || capital(words(s.kind.replace(/^builded_/, '')));
const zoneName = z => z.label || capital(words(z.name || ''));
const stat = (p, k) => num(p.stats[k]);

// ---------- theme
// Colors come from the CSS variables of the current theme.
let P = {};
const css = name => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
function readPalette() {
  P = {
    team: { 0: css('--team0'), 1: css('--team1'), 2: css('--team2') },
    text: css('--text'), muted: css('--muted'), line: css('--line'), select: css('--select'),
    panel: css('--panel'), raised: css('--raised'), mapbg: css('--mapbg'), label: css('--label'), labelText: css('--label-text'),
    head: css('--head'), body: css('--body'), dark: document.documentElement.dataset.theme === 'dark',
  };
}
const teamColor = t => P.team[t] || P.team[0];
const THEME_KEY = 'erpl-report-theme';
let ready = false;
function applyTheme(t) {
  document.documentElement.dataset.theme = t;
  try { localStorage.setItem(THEME_KEY, t); } catch (e) { /* storage may be unavailable */ }
  readPalette();
  sprites.clear();
  $('theme').innerHTML = icon(t === 'dark' ? 'sun' : 'moon');
  $('theme').title = tr(t === 'dark' ? 'themeLight' : 'themeDark');
  if (ready) { renderTop(); renderPanel(); }
  draw();
}

// ---------- state
// mode is what the map shows: infantry, vehicles or structures. team filters it; sel is
// the vehicle or structure picked in the list or on the map.
const state = { mode: 'fight', team: 0, t0: 0, t1: END, sel: null, hot: null, skipSpawn: true, deaths: false, planes: false, other: false, heat: false };
const MODES = [['fight', 'flame', 'modeFight'], ['vehicles', 'car', 'modeVehicles'], ['structures', 'hammer', 'modeStructures']];
const same = (a, b) => !!(a && b && a.type === b.type && a.id === b.id);
const ofTeam = t => !state.team || t === state.team;
const whole = () => state.t0 <= 0 && state.t1 >= END;

// ---------- time and sectors
const overlaps = (a, b) => a <= state.t1 && b >= state.t0;
// zoneActive tells whether the zone was active for a while within the chosen time; zones
// without activity data are active all the time.
const zoneActive = z => !z.active || z.active.some(([a, b]) => a < state.t1 && b > state.t0);
// ownerAt is the owner after the last change at or before t: a team, -1 for nobody, or undefined.
function ownerAt(z, t) {
  let o;
  for (const [ct, tm] of z.owners || []) if (ct <= t) o = tm;
  return o;
}
// posAt returns the last track point at or before t.
function posAt(track, t) {
  let p = null;
  for (let i = 0; i < track.length && track[i] <= t; i += 3) p = [track[i + 1], track[i + 2]];
  return p;
}

// SECTORS are the stages of the battle: zones of a group are active together, and the
// battle moves from one group to the next. Each has its span and who took it.
const SECTORS = (() => {
  const groups = new Map();
  D.zones.forEach(z => {
    if (!z.active || !z.active.length) return;
    const key = z.group || z.name;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(z);
  });
  const list = [...groups].map(([group, zones]) => {
    const spans = zones.flatMap(z => z.active);
    const a = Math.min(...spans.map(s => s[0])), b = Math.max(...spans.map(s => s[1]));
    const before = ownerAt(zones[0], a + 0.5), after = ownerAt(zones[0], b);
    return { group, zones, a, b, taker: after > 0 && after !== before ? after : 0, holder: before > 0 ? before : 0 };
  }).sort((x, y) => x.a - y.a);
  list.forEach((s, i) => { s.n = i + 1; s.name = s.zones.map(z => z.title || zoneName(z)).join(', '); });
  return list;
})();
const sectorOf = group => SECTORS.find(s => s.group === group);
// areaActive tells whether a battle area was in force in the chosen time. An area of a
// sector is in force while the sector goes on; areas of sectors that never opened (the
// game picks some of the possible sectors for a battle) never are. An area without a
// group is in force all the time, and so are all areas if no group matches a sector.
const areaGroupsKnown = (D.areas || []).some(a => a.group && sectorOf(a.group));
function areaActive(a) {
  if (!a.group || !areaGroupsKnown) return true;
  const s = sectorOf(a.group);
  return !!s && s.a < state.t1 && s.b > state.t0;
}

// ---------- world and view
// Map units: the map image is S x S units; without a map the fighting area is.
const S = 1024;
const allSoldiers = D.soldiers.map((s, i) => i);
let L, T, R, B;
if (D.map) {
  [L, T] = D.map.leftTop;
  [R, B] = D.map.rightBottom;
} else {
  const b = box(allSoldiers, 0.01, 0.99);
  const side = Math.max(b.r - b.l, b.t - b.b, 200) * 1.3, cx = (b.l + b.r) / 2, cz = (b.t + b.b) / 2;
  [L, T, R, B] = [cx - side / 2, cz + side / 2, cx + side / 2, cz - side / 2];
}
const mx = x => (x - L) / (R - L) * S;
const my = z => (z - T) / (B - T) * S;
const metersPerUnit = (R - L) / S;
const MAX_ZOOM = 40;

// box is the box around the positions of the given soldiers between two quantiles.
function box(soldiers, qlo, qhi) {
  const xs = [], zs = [];
  for (const i of soldiers) {
    const tr_ = D.soldiers[i].track;
    for (let j = 0; j < tr_.length; j += 3) { xs.push(tr_[j + 1]); zs.push(tr_[j + 2]); }
  }
  if (!xs.length) return { l: -100, r: 100, t: 100, b: -100 };
  xs.sort((a, b) => a - b); zs.sort((a, b) => a - b);
  const q = (a, f) => a[Math.min(a.length - 1, Math.floor(f * (a.length - 1)))];
  return { l: q(xs, qlo), r: q(xs, qhi), b: q(zs, qlo), t: q(zs, qhi) };
}

const cv = $('map'), ctx = cv.getContext('2d');
const view = { k: 1, ox: 0, oy: 0 };
let W = 0, H = 0, DPR = 1;
let hovered = null; // the vehicle route point under the pointer: {i, t, x, z}
const sx = x => mx(x) * view.k + view.ox;
const sy = z => my(z) * view.k + view.oy;

function fitWorld(b, pad = 1.25) {
  const x0 = mx(b.l), x1 = mx(b.r), y0 = my(b.t), y1 = my(b.b);
  const bw = Math.max(x1 - x0, 15) * pad, bh = Math.max(y1 - y0, 15) * pad;
  view.k = Math.min(MAX_ZOOM, Math.max(0.15, Math.min(W / bw, H / bh)));
  view.ox = W / 2 - (x0 + x1) / 2 * view.k;
  view.oy = H / 2 - (y0 + y1) / 2 * view.k;
  draw();
}
const fitBattle = () => fitWorld(box(allSoldiers, 0.02, 0.98));
function fitPoints(pts, minSide = 150) {
  if (!pts.length) return;
  const xs = pts.map(p => p[0]), zs = pts.map(p => p[1]);
  const cx = (Math.min(...xs) + Math.max(...xs)) / 2, cz = (Math.min(...zs) + Math.max(...zs)) / 2;
  const hw = Math.max(Math.max(...xs) - Math.min(...xs), minSide) / 2, hh = Math.max(Math.max(...zs) - Math.min(...zs), minSide) / 2;
  fitWorld({ l: cx - hw, r: cx + hw, b: cz - hh, t: cz + hh }, 1.2);
}

// ---------- heatmap
// The grid covers where almost all soldiers were (pilots fly far away) and has at most
// about 600 cells a side, so it is rebuilt at once when a filter changes.
const heat = { key: '', canvas: document.createElement('canvas') };
(function heatGrid() {
  const b = box(allSoldiers, 0.01, 0.99), pad = 150;
  const wm = b.r - b.l + 2 * pad, hm = b.t - b.b + 2 * pad;
  heat.cell = Math.max(3, Math.max(wm, hm) / 600);
  heat.l = b.l - pad; heat.t = b.t + pad;
  heat.w = Math.max(1, Math.ceil(wm / heat.cell));
  heat.h = Math.max(1, Math.ceil(hm / heat.cell));
  heat.canvas.width = heat.w; heat.canvas.height = heat.h;
})();

function buildHeat() {
  const { w, h, cell } = heat;
  const g = new Float32Array(w * h);
  for (const s of D.soldiers) {
    if (!ofTeam(s.team)) continue;
    const tr_ = s.track;
    for (let i = 0; i < tr_.length; i += 3) {
      const t = tr_[i];
      if (t < state.t0 || t > state.t1 || (state.skipSpawn && t - s.spawned < 10)) continue;
      const cx = Math.floor((tr_[i + 1] - heat.l) / cell), cy = Math.floor((heat.t - tr_[i + 2]) / cell);
      if (cx >= 0 && cy >= 0 && cx < w && cy < h) g[cy * w + cx]++;
    }
  }
  blur(g, w, h, 2); blur(g, w, h, 2);
  const vals = [];
  for (const v of g) if (v > 0.01) vals.push(v);
  vals.sort((a, b) => a - b);
  const top = vals.length ? vals[Math.floor(vals.length * 0.995)] : 1;
  const img = new ImageData(w, h);
  for (let i = 0; i < g.length; i++) {
    const v = Math.min(1, g[i] / top);
    if (v < 0.03) continue;
    const [r, gg, b] = ramp(v);
    img.data.set([r, gg, b, Math.round(255 * Math.min(0.8, 0.1 + v))], i * 4);
  }
  heat.canvas.getContext('2d').putImageData(img, 0, 0);
}

// blur is a separable box blur of radius r.
function blur(g, w, h, r) {
  const tmp = new Float32Array(g.length), n = 2 * r + 1;
  for (let y = 0; y < h; y++) {
    let acc = 0;
    for (let x = -r; x <= r; x++) acc += g[y * w + Math.min(w - 1, Math.max(0, x))];
    for (let x = 0; x < w; x++) {
      tmp[y * w + x] = acc / n;
      acc += g[y * w + Math.min(w - 1, x + r + 1)] - g[y * w + Math.max(0, x - r)];
    }
  }
  for (let x = 0; x < w; x++) {
    let acc = 0;
    for (let y = -r; y <= r; y++) acc += tmp[Math.min(h - 1, Math.max(0, y)) * w + x];
    for (let y = 0; y < h; y++) {
      g[y * w + x] = acc / n;
      acc += tmp[Math.min(h - 1, y + r + 1) * w + x] - tmp[Math.max(0, y - r) * w + x];
    }
  }
}

function ramp(v) {
  const stops = [[0, [255, 241, 184]], [0.3, [255, 209, 102]], [0.65, [243, 114, 44]], [1, [214, 40, 40]]];
  for (let i = 1; i < stops.length; i++) {
    const [p1, c1] = stops[i], [p0, c0] = stops[i - 1];
    if (v <= p1) {
      const f = (v - p0) / (p1 - p0);
      return c0.map((c, j) => Math.round(c + (c1[j] - c) * f));
    }
  }
  return stops[stops.length - 1][1];
}

// ---------- markers
// A marker is a Lucide icon on a disc of the team color. The same SVG is used on the
// map and in the legend, so they look alike.
function markerSVG(name, color, px) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 32 32">` +
    `<circle cx="16" cy="16" r="14.6" fill="${color}" stroke="#ffffff" stroke-width="2"/>` +
    `<g transform="translate(8 8) scale(.6667)" fill="none" stroke="#ffffff" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round">${ICONS[name]}</g></svg>`;
}
const markerURL = (name, color, px) => 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(markerSVG(name, color, px));
const sprites = new Map();
function marker(name, color, size, x, y, alpha = 1) {
  const key = `${name}|${color}|${size}|${DPR}`;
  let img = sprites.get(key);
  if (!img) {
    img = new Image();
    img.onload = draw;
    img.src = markerURL(name, color, Math.round(size * DPR));
    sprites.set(key, img);
  }
  if (!img.complete || !img.naturalWidth) return;
  ctx.globalAlpha = alpha;
  ctx.drawImage(img, Math.round(x - size / 2), Math.round(y - size / 2), size, size);
  ctx.globalAlpha = 1;
}

// ---------- drawing
const mapImg = new Image();
if (D.map) { mapImg.onload = () => draw(); mapImg.src = D.map.image; }
let frame = 0;
function draw() { if (!frame) frame = requestAnimationFrame(render); }

function path(points) {
  ctx.beginPath();
  points.forEach(([x, z], i) => i ? ctx.lineTo(sx(x), sy(z)) : ctx.moveTo(sx(x), sy(z)));
  ctx.closePath();
}
// stroke draws the current path over a dark casing, so lines stand out on any map.
function stroke(color, width, alpha = 1, casing = 'rgba(15,17,19,.55)') {
  ctx.lineJoin = 'round'; ctx.lineCap = 'round';
  ctx.globalAlpha = alpha;
  ctx.strokeStyle = casing; ctx.lineWidth = width + 2; ctx.stroke();
  ctx.strokeStyle = color; ctx.lineWidth = width; ctx.stroke();
  ctx.globalAlpha = 1;
}

// shownVehicles are the vehicles of the chosen team in the battle in the chosen time.
function shownVehicles() {
  return D.vehicles.map((v, i) => [v, i]).filter(([v]) => v.track.length && ofTeam(v.team) && v.player >= 0 &&
    (state.planes || v.kind !== 'plane') && overlaps(v.spawned, v.destroyed || END));
}
// shownStructures are the structures of the chosen team built in the chosen time.
function shownStructures() {
  return D.structures.map((s, i) => [s, i]).filter(([s]) => (s.rally || state.other) && ofTeam(s.team) && s.built >= state.t0 && s.built <= state.t1)
    .sort((a, b) => a[0].built - b[0].built);
}
function vehiclePath(v) {
  ctx.beginPath();
  let started = false;
  for (let i = 0; i < v.track.length; i += 3) {
    const t = v.track[i];
    if (t < state.t0 || t > state.t1) { started = false; continue; }
    const x = sx(v.track[i + 1]), y = sy(v.track[i + 2]);
    started ? ctx.lineTo(x, y) : ctx.moveTo(x, y);
    started = true;
  }
}
// vehiclePoint is the first route point at or after t (dir 1) or the last at or before t
// (dir -1), within the chosen time.
function vehiclePoint(v, t, dir) {
  const tr_ = v.track;
  let p = null;
  for (let i = 0; i < tr_.length; i += 3) {
    const pt = tr_[i];
    if (pt < state.t0 || pt > state.t1) continue;
    if (dir > 0 && pt >= t) return [tr_[i + 1], tr_[i + 2]];
    if (dir < 0 && pt <= t) p = [tr_[i + 1], tr_[i + 2]];
  }
  return p;
}

let deathCache = null;
// deaths lists the deaths in the chosen time with their places.
function deaths() {
  if (!deathCache) deathCache = D.soldiers.map(s => {
    if (!s.died) return null;
    const p = posAt(s.track, s.died);
    return p && { t: s.died, x: p[0], z: p[1], team: s.team, player: s.player, killer: s.killer };
  }).filter(Boolean);
  return deathCache.filter(d => d.t >= state.t0 && d.t <= state.t1 && ofTeam(d.team));
}

function render() {
  frame = 0;
  ctx.setTransform(DPR, 0, 0, DPR, 0, 0);
  ctx.fillStyle = P.mapbg;
  ctx.fillRect(0, 0, W, H);
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = 'high';
  if (D.map && mapImg.complete && mapImg.naturalWidth) {
    ctx.drawImage(mapImg, view.ox, view.oy, S * view.k, S * view.k);
    if (P.dark) { ctx.fillStyle = 'rgba(15,17,19,.3)'; ctx.fillRect(view.ox, view.oy, S * view.k, S * view.k); }
  } else grid();

  // battle areas only for a part of the battle: over the whole battle they pile up
  if (!whole()) {
    ctx.setLineDash([10, 6]);
    for (const a of D.areas || []) {
      if (a.air || !areaActive(a) || !(a.team <= 0 || ofTeam(a.team))) continue;
      path(a.points);
      stroke(a.team > 0 ? teamColor(a.team) : P.text, 1.25, 0.75);
    }
    ctx.setLineDash([]);
  }

  const zones = D.zones.filter(zoneActive);
  for (const z of zones) {
    const owner = ownerAt(z, state.t1), color = owner > 0 ? teamColor(owner) : P.label;
    zoneShape(z);
    ctx.fillStyle = color; ctx.globalAlpha = state.mode === 'fight' ? 0.12 : 0.2; ctx.fill(); ctx.globalAlpha = 1;
    stroke(color, 1.5);
  }

  if (state.mode === 'fight' || state.heat) drawHeat();
  if (state.mode === 'fight') renderDeaths();
  else if (state.mode === 'vehicles') renderVehicles();
  else renderStructures();

  for (const z of zones) zoneLabel(z);
  scaleBar();
  timeline.render();
}

function drawHeat() {
  const key = [state.team, state.skipSpawn, state.t0, state.t1].join('|');
  if (key !== heat.key) { buildHeat(); heat.key = key; }
  const s = heat.cell / metersPerUnit * view.k;
  ctx.drawImage(heat.canvas, sx(heat.l), sy(heat.t), heat.w * s, heat.h * s);
}

function renderDeaths() {
  if (state.deaths) for (const d of deaths()) {
    ctx.beginPath(); ctx.arc(sx(d.x), sy(d.z), 2.6, 0, Math.PI * 2);
    ctx.fillStyle = teamColor(d.team); ctx.fill();
    ctx.lineWidth = 1.2; ctx.strokeStyle = '#fff'; ctx.stroke();
  }
}

function renderVehicles() {
  const vs = shownVehicles(), focus = state.hot || state.sel;
  const dim = i => !focus || focus.id === i ? 1 : 0.3;
  for (const [v, i] of vs) {
    const plane = v.kind === 'plane', w = plane ? 1 : 1.75;
    vehiclePath(v);
    if ((focus && focus.id === i) || (hovered && hovered.i === i)) stroke(P.select, w + 3, 0.9, 'transparent');
    stroke(teamColor(v.team), w, dim(i) * (plane ? 0.75 : 0.95));
  }
  for (const [v, i] of vs) {
    const first = vehiclePoint(v, state.t0, 1);
    if (first) marker(v.kind === 'plane' ? 'plane' : 'car', teamColor(v.team), 18, sx(first[0]), sy(first[1]), dim(i));
    if (v.destroyed && v.destroyed >= state.t0 && v.destroyed <= state.t1) {
      const p = vehiclePoint(v, v.destroyed, -1);
      if (p) marker('x', teamColor(v.team), 16, sx(p[0]), sy(p[1]), dim(i));
    }
  }
  if (hovered) {
    ctx.beginPath(); ctx.arc(sx(hovered.x), sy(hovered.z), 5, 0, Math.PI * 2);
    ctx.fillStyle = '#fff'; ctx.fill(); ctx.lineWidth = 2; ctx.strokeStyle = '#111'; ctx.stroke();
  }
}

function renderStructures() {
  const focus = state.hot || state.sel;
  for (const [s, i] of shownStructures()) {
    const on = focus && focus.id === i, a = !focus || on ? 1 : 0.35, size = (s.rally ? 20 : 15) + (on ? 4 : 0);
    if (on) { ctx.beginPath(); ctx.arc(sx(s.pos[0]), sy(s.pos[1]), size / 2 + 4, 0, Math.PI * 2); ctx.fillStyle = P.select; ctx.globalAlpha = 0.85; ctx.fill(); ctx.globalAlpha = 1; }
    marker(s.rally ? 'tent' : 'hammer', teamColor(s.team), size, sx(s.pos[0]), sy(s.pos[1]), a);
  }
}

function grid() {
  const step = 100 / metersPerUnit * view.k;
  ctx.strokeStyle = P.line; ctx.lineWidth = 1;
  ctx.beginPath();
  for (let v = 0; v <= S * view.k + 0.5; v += step) {
    ctx.moveTo(view.ox + v, view.oy); ctx.lineTo(view.ox + v, view.oy + S * view.k);
    ctx.moveTo(view.ox, view.oy + v); ctx.lineTo(view.ox + S * view.k, view.oy + v);
  }
  ctx.stroke();
}

function zoneShape(z) {
  if (z.area && z.area.length > 2) path(z.area);
  else { ctx.beginPath(); ctx.arc(sx(z.pos[0]), sy(z.pos[1]), Math.max(z.radius || 10, 4) / metersPerUnit * view.k, 0, Math.PI * 2); }
}

// zoneLabel draws the zone's letter in a disc, or its name in a pill, edged in the owner's color.
function zoneLabel(z) {
  const owner = ownerAt(z, state.t1), color = owner > 0 ? teamColor(owner) : P.muted;
  const x = sx(z.pos[0]), y = sy(z.pos[1]), text = z.title || zoneName(z);
  ctx.font = `600 ${z.title ? 13 : 11.5}px ${P.head}`;
  const w = z.title ? 22 : ctx.measureText(text).width + 14, h = z.title ? 22 : 19;
  ctx.beginPath(); ctx.roundRect(x - w / 2, y - h / 2, w, h, h / 2);
  ctx.fillStyle = P.label; ctx.fill();
  ctx.lineWidth = 1.75; ctx.strokeStyle = color; ctx.stroke();
  ctx.fillStyle = P.labelText; ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
  ctx.fillText(text, x, y + 1);
}

function scaleBar() {
  const target = 110 / view.k * metersPerUnit;
  const m = [2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000].find(v => v >= target * 0.6) || 5000;
  $('scale').innerHTML = `<i style="width:${m / metersPerUnit * view.k}px"></i>${tr('meters', m)}`;
}

// ---------- map pointer
function zoomAt(f, px, py) {
  const k = Math.min(MAX_ZOOM, Math.max(0.15, view.k * f));
  view.ox = px - (px - view.ox) * k / view.k;
  view.oy = py - (py - view.oy) * k / view.k;
  view.k = k;
  draw();
}
cv.addEventListener('wheel', e => { e.preventDefault(); zoomAt(Math.exp(-e.deltaY * 0.0015), e.offsetX, e.offsetY); }, { passive: false });
let drag = null;
cv.addEventListener('pointerdown', e => { drag = { x: e.clientX, y: e.clientY, ox: view.ox, oy: view.oy, moved: false }; cv.setPointerCapture(e.pointerId); });
cv.addEventListener('pointermove', e => {
  if (!drag) return hover(e.offsetX, e.offsetY);
  if (Math.hypot(e.clientX - drag.x, e.clientY - drag.y) > 3) drag.moved = true;
  if (!drag.moved) return;
  cv.classList.add('drag');
  view.ox = drag.ox + e.clientX - drag.x;
  view.oy = drag.oy + e.clientY - drag.y;
  $('tip').style.display = 'none';
  draw();
});
cv.addEventListener('pointerup', e => {
  if (drag && !drag.moved) {
    const hit = pick(e.offsetX, e.offsetY);
    select(hit && hit.obj ? hit.obj : null, false);
  }
  drag = null; cv.classList.remove('drag');
});
cv.addEventListener('pointerleave', () => { $('tip').style.display = 'none'; markRow(null); if (hovered) { hovered = null; draw(); } });
$('zoomIn').onclick = () => zoomAt(1.5, W / 2, H / 2);
$('zoomOut').onclick = () => zoomAt(1 / 1.5, W / 2, H / 2);
$('zoomFit').onclick = fitBattle;

const colored = (text, t) => `<span style="color:${teamColor(t)}">${esc(text)}</span>`;
// pick finds what is under the pointer in the current mode: {obj, html, route} or null.
function pick(px, py) {
  const near = (x, z, r) => Math.hypot(sx(x) - px, sy(z) - py) <= r;
  for (const z of D.zones.filter(zoneActive)) if (near(z.pos[0], z.pos[1], 15)) return { html: zoneTip(z) };
  if (state.mode === 'structures') {
    for (const [s, i] of shownStructures()) {
      if (!near(s.pos[0], s.pos[1], s.rally ? 13 : 10)) continue;
      return { obj: { type: 'structure', id: i }, html: structureTip(s) };
    }
  } else if (state.mode === 'vehicles') {
    let best = 10, hit = null;
    for (const [v, i] of shownVehicles()) {
      const tr_ = v.track;
      for (let j = 0; j < tr_.length; j += 3) {
        if (tr_[j] < state.t0 || tr_[j] > state.t1) continue;
        const d = Math.hypot(sx(tr_[j + 1]) - px, sy(tr_[j + 2]) - py);
        if (d < best) { best = d; hit = { i, t: tr_[j], x: tr_[j + 1], z: tr_[j + 2] }; }
      }
    }
    if (hit) {
      const v = D.vehicles[hit.i];
      return {
        obj: { type: 'vehicle', id: hit.i }, route: hit,
        html: `<b>${esc(vehicleName(v))}</b><div>${colored(playerName(v.player), v.team)}</div>` +
          `<div>${tr('hereAt', clock(hit.t))}</div><div class="m">${vehicleLife(v)}</div>`,
      };
    }
  } else if (state.deaths) {
    for (const d of deaths()) {
      if (!near(d.x, d.z, 6)) continue;
      return {
        html: `<b>${colored(playerName(d.player), d.team)}</b><div class="m">` +
          (d.killer >= 0 && d.killer !== d.player ? tr('killedBy', clock(d.t), colored(playerName(d.killer), D.players[d.killer].team)) : tr('killedAt', clock(d.t))) + '</div>',
      };
    }
  }
  return null;
}
const vehicleLife = v => v.destroyed ? tr('vehicleLife', clock(v.spawned), clock(v.destroyed)) : tr('vehicleLifeEnd', clock(v.spawned));
function structureTip(s) {
  return `<b>${esc(s.rally ? tr('rallyPoint') : structureName(s))}</b>` +
    `<div>${tr('builtBy', colored(playerName(s.player), s.team), clock(s.built))}</div>` +
    `<div class="m">${s.removed ? tr('destroyedAt', clock(s.removed)) : tr('notDestroyed')}</div>`;
}
function zoneTip(z) {
  const sec = SECTORS.find(s => s.zones.includes(z));
  const owners = (z.owners || []).map(([t, o]) => `${clock(t)} ${o > 0 ? colored(tr('team', o), o) : tr('nobody')}`).join(', ');
  return `<b>${esc(z.title ? `${z.title}, ${zoneName(z)}` : zoneName(z))}</b>` +
    (sec ? `<div>${tr('sectorSpan', sec.n, clock(sec.a), clock(sec.b))}</div>` : '') +
    (owners ? `<div class="m">${tr('owners')}: ${owners}</div>` : '');
}

function hover(px, py) {
  const hit = pick(px, py);
  const route = hit && hit.route || null;
  if ((route ? route.i + ':' + route.t : '') !== (hovered ? hovered.i + ':' + hovered.t : '')) { hovered = route; draw(); }
  cv.classList.toggle('point', !!(hit && hit.obj));
  markRow(hit && hit.obj);
  const tip = $('tip');
  if (!hit) { tip.style.display = 'none'; return; }
  tip.innerHTML = hit.html;
  tip.style.display = 'block';
  tip.style.left = Math.min(px + 16, W - tip.offsetWidth - 8) + 'px';
  tip.style.top = Math.min(py + 16, H - tip.offsetHeight - 8) + 'px';
}
const rowSelector = obj => `#panel [data-type="${obj.type}"][data-id="${obj.id}"]`;
function markRow(obj) {
  document.querySelectorAll('#panel .hot').forEach(e => e.classList.remove('hot'));
  const row = obj && document.querySelector(rowSelector(obj));
  if (row) row.classList.add('hot');
}

// select makes obj the selection, or clears it when it is already selected.
function select(obj, fit) {
  state.sel = obj && !same(obj, state.sel) ? obj : null;
  state.hot = null;
  renderPanel();
  if (fit && state.sel) {
    if (state.sel.type === 'vehicle') {
      const tr_ = D.vehicles[state.sel.id].track, pts = [];
      for (let i = 0; i < tr_.length; i += 3) pts.push([tr_[i + 1], tr_[i + 2]]);
      fitPoints(pts);
    } else fitPoints([D.structures[state.sel.id].pos]);
  }
  const row = state.sel && document.querySelector(rowSelector(state.sel));
  if (row) row.scrollIntoView({ block: 'nearest' });
  draw();
}

// ---------- header
const killsOf = n => D.players.filter(p => p.team === n).reduce((a, p) => a + stat(p, 'kills'), 0);
function renderTop() {
  const m = D.meta;
  $('mission').textContent = m.missionName || capital(words(m.mission));
  const date = m.started ? new Date(m.started).toLocaleString(LANG_CODE, { dateStyle: 'long', timeStyle: 'short' }) : '';
  const mode = [m.modeName || capital(words(m.mode || '')), m.campaignName || capital(words(m.campaign || ''))].filter(Boolean).join(', ');
  const minutes = Math.floor(m.duration / 60), seconds = Math.floor(m.duration % 60);
  $('subtitle').textContent = [mode, date, tr('lasted', tr('minutesSeconds', minutes, seconds))].filter(Boolean).join('. ') + '.';
  $('teams').innerHTML = D.teams.map(t => {
    const k = killsOf(t.number), n = D.players.filter(p => p.team === t.number).length;
    return `<div class="teamchip" style="--c:${teamColor(t.number)}"><b>${esc(teamName(t.number))}</b>` +
      `<span>${tr('nPlayers', n)}, ${k} ${tr('killsWord', k)}</span></div>`;
  }).join('');
  $('modes').innerHTML = MODES.map(([id, ic, key]) =>
    `<button role="tab" data-mode="${id}" aria-selected="${state.mode === id}">${icon(ic)}${tr(key)}</button>`).join('');
}
$('modes').addEventListener('click', e => {
  const b = e.target.closest('button');
  if (!b || b.dataset.mode === state.mode) return;
  state.mode = b.dataset.mode; state.sel = null; state.hot = null; hovered = null;
  renderTop(); renderPanel(); draw();
});

// ---------- panel of the mode
const svgLine = (color, dash = '') => `<svg width="34" height="10" aria-hidden="true"><line x1="2" y1="5" x2="32" y2="5" stroke="rgba(15,17,19,.55)" stroke-width="6" stroke-linecap="round"${dash}/><line x1="2" y1="5" x2="32" y2="5" stroke="${color}" stroke-width="3" stroke-linecap="round"${dash}/></svg>`;
const svgDot = color => `<svg width="12" height="12" aria-hidden="true"><circle cx="6" cy="6" r="4.2" fill="${color}" stroke="#fff" stroke-width="1.4"/></svg>`;
const svgZone = () => `<svg width="34" height="20" aria-hidden="true"><rect x="3" y="3" width="28" height="14" rx="3" fill="${P.team[1]}" fill-opacity=".2" stroke="${P.team[1]}" stroke-width="2.5"/></svg>`;
const img = (name, color) => `<img src="${markerURL(name, color, 40)}" alt="">`;
const both = fn => D.teams.map(t => fn(teamColor(t.number))).join('');
const legendItem = (sym, text, note) => `<li><span class="sym">${sym}</span><span>${text}${note ? `<small>${note}</small>` : ''}</span></li>`;

function renderPanel() {
  const teamSeg = `<div class="seg" id="teamSeg">` +
    [[0, tr('bothTeams'), null], ...D.teams.map(t => [t.number, tr('team', t.number), teamColor(t.number)])]
      .map(([v, l, c]) => `<button data-v="${v}" aria-pressed="${v === state.team}">${c ? `<i style="--c:${c}"></i>` : ''}${esc(l)}</button>`).join('') + '</div>';
  const sw = (id, on, text) => `<label class="switchrow"><span>${text}</span><input type="checkbox" class="switch" data-opt="${id}"${on ? ' checked' : ''}></label>`;
  let intro = '', legend = '', options = '', list = '';
  const heatLegend = legendItem('<span class="ramp"></span>', tr('legendHeat'), tr('legendHeatNote'));
  const zoneLegend = legendItem(svgZone(), tr('legendZone'), tr('legendZoneNote')) +
    (whole() ? '' : legendItem(both(c => svgLine(c, ' stroke-dasharray="7 5"')), tr('legendArea')));

  if (state.mode === 'fight') {
    intro = tr('fightIntro');
    legend = heatLegend +
      (state.deaths ? legendItem(both(svgDot), tr('legendDeath')) : '') + zoneLegend;
    options = sw('skipSpawn', state.skipSpawn, tr('skipSpawn')) + sw('deaths', state.deaths, tr('showDeaths'));
    list = SECTORS.length ? `<h3>${tr('fightList')}</h3><div class="list">` + SECTORS.map(s => {
      const lost = t => D.soldiers.filter(x => x.died >= s.a && x.died <= s.b && x.team === t).length;
      const res = s.taker ? tr('sectorTaken', team(s.taker) && team(s.taker).army ? armyName(team(s.taker)) : s.taker) : tr('sectorHeld');
      return `<button class="item" data-type="sector" data-id="${s.n}" style="--c:${s.taker ? teamColor(s.taker) : P.muted}"${state.t0 === s.a && state.t1 === s.b ? ' aria-current="true"' : ''}>` +
        `<span class="ico">${s.n}</span><span class="t">${esc(s.name)}</span><span class="r">${span(s.a, s.b)}</span>` +
        `<span class="s">${esc(res)}</span><span class="r m">${tr('losses')} ${D.teams.map(t => colored(lost(t.number), t.number)).join(' : ')}</span></button>`;
    }).join('') + '</div>' : '';
  } else if (state.mode === 'vehicles') {
    intro = tr('vehiclesIntro');
    legend = legendItem(both(c => svgLine(c)), tr('legendRoute')) +
      legendItem(both(c => img('car', c)), tr('legendCalled')) +
      legendItem(both(c => img('x', c)), tr('legendKnocked')) + (state.heat ? heatLegend : '') + zoneLegend;
    options = sw('planes', state.planes, tr('showPlanes')) + sw('heat', state.heat, tr('showHeat'));
    const vs = D.vehicles.map((v, i) => [v, i]).filter(([v]) => v.player >= 0 && ofTeam(v.team) && (state.planes || v.kind !== 'plane'));
    list = `<h3>${tr('vehiclesList', vs.length)}</h3>` + (vs.length ? '<div class="list">' + vs.map(([v, i]) =>
      `<button class="item" data-type="vehicle" data-id="${i}" style="--c:${teamColor(v.team)}"${same(state.sel, { type: 'vehicle', id: i }) ? ' aria-current="true"' : ''}>` +
      `<span class="ico">${icon(v.kind === 'plane' ? 'plane' : 'car')}</span><span class="t" title="${esc(v.model)}">${esc(vehicleName(v))}</span>` +
      `<span class="r">${span(v.spawned, v.destroyed)}</span>` +
      `<span class="s">${esc(playerName(v.player))}</span><span class="r ${v.destroyed ? 'bad' : 'good'}">${v.destroyed ? tr('knockedOut') : tr('survived')}</span></button>`).join('') + '</div>'
      : `<p class="empty">${tr('noVehicles')}</p>`);
  } else {
    intro = tr('structuresIntro');
    legend = legendItem(both(c => img('tent', c)), tr('legendRally')) +
      (state.other ? legendItem(both(c => img('hammer', c)), tr('legendOther')) : '') + (state.heat ? heatLegend : '') + zoneLegend;
    options = sw('other', state.other, tr('showOther')) + sw('heat', state.heat, tr('showHeat'));
    const ss = shownStructures();
    list = `<h3>${tr(whole() ? 'structuresList' : 'structuresListPart', ss.length)}</h3>` + (ss.length ? '<div class="list">' + ss.map(([s, i]) =>
      `<button class="item" data-type="structure" data-id="${i}" style="--c:${teamColor(s.team)}"${same(state.sel, { type: 'structure', id: i }) ? ' aria-current="true"' : ''}>` +
      `<span class="ico">${icon(s.rally ? 'tent' : 'hammer')}</span><span class="t">${esc(s.rally ? tr('rallyPoint') : structureName(s))}</span>` +
      `<span class="r">${span(s.built, s.removed)}</span>` +
      `<span class="s">${esc(playerName(s.player))}</span><span class="r ${s.removed ? 'bad' : 'good'}">${s.removed ? tr('destroyed') : tr('standing')}</span></button>`).join('') + '</div>'
      : `<p class="empty">${tr('noStructures')}</p>`);
  }
  const title = MODES.find(m => m[0] === state.mode)[2];
  $('panel').innerHTML = `<div class="block"><h2>${tr(title)}</h2><p class="lead">${intro}</p></div>` +
    `<div class="block">${teamSeg}${options}</div>` +
    `<div class="block"><h3>${tr('legend')}</h3><ul class="legend">${legend}</ul></div>` +
    (list ? `<div class="block">${list}</div>` : '');
}

$('panel').addEventListener('click', e => {
  const seg = e.target.closest('#teamSeg button');
  if (seg) { state.team = +seg.dataset.v; state.sel = null; renderPanel(); draw(); return; }
  const row = e.target.closest('[data-type]');
  if (!row) return;
  if (row.dataset.type === 'sector') {
    const s = SECTORS[+row.dataset.id - 1];
    if (state.t0 === s.a && state.t1 === s.b) setRange(0, END);
    else { setRange(s.a, s.b); fitPoints(s.zones.map(z => z.pos), 300); }
    return;
  }
  select({ type: row.dataset.type, id: +row.dataset.id }, true);
});
$('panel').addEventListener('change', e => {
  const opt = e.target.dataset.opt;
  if (!opt) return;
  state[opt] = e.target.checked;
  renderPanel(); draw();
});
$('panel').addEventListener('mouseover', e => {
  const row = e.target.closest('[data-type="vehicle"], [data-type="structure"]');
  const obj = row ? { type: row.dataset.type, id: +row.dataset.id } : null;
  if (!same(obj, state.hot) && (obj || state.hot)) { state.hot = obj; draw(); }
});
$('panel').addEventListener('mouseleave', () => { if (state.hot) { state.hot = null; draw(); } });

// ---------- battle course
// The sectors of the battle as a row of stages, the kills of each team in 10-second
// buckets under it, and a time axis. A click on a sector shows it; dragging selects any
// part of the battle.
const timeline = (() => {
  const tc = $('timeline'), g = tc.getContext('2d');
  tc.title = tr(SECTORS.length ? 'stripHint' : 'stripHintNoSectors');
  const BUCKET = 10, LEFT = 14, RIGHT = 14, SECT = 30, KILLS = 40, AXIS = 22;
  const n = Math.max(1, Math.ceil(END / BUCKET));
  const kills = { 1: new Array(n).fill(0), 2: new Array(n).fill(0) };
  for (const s of D.soldiers) {
    if (!s.died || s.killer < 0) continue;
    const kt = D.players[s.killer].team;
    if (kills[kt] && kt !== s.team) kills[kt][Math.min(n - 1, Math.floor(s.died / BUCKET))]++;
  }
  const peak = Math.max(1, ...kills[1], ...kills[2]);
  const top = SECTORS.length ? SECT + 8 : 0, height = 6 + top + KILLS + AXIS;

  let w = 0, dragFrom = null, cursor = null;
  const xOf = t => LEFT + t / END * (w - LEFT - RIGHT);
  const tOf = x => Math.min(END, Math.max(0, (x - LEFT) / (w - LEFT - RIGHT) * END));
  const sectorAt = (x, y) => y >= 6 && y < 6 + SECT ? SECTORS.find(s => { const t = tOf(x); return t >= s.a && t <= s.b; }) : null;

  function resize() {
    w = tc.clientWidth;
    tc.style.height = height + 'px';
    tc.width = w * DPR; tc.height = height * DPR;
  }
  function render() {
    g.setTransform(DPR, 0, 0, DPR, 0, 0);
    g.clearRect(0, 0, w, height);
    const inSel = (a, b) => b > state.t0 && a < state.t1;
    g.textBaseline = 'middle';

    // sectors
    for (const s of SECTORS) {
      const x0 = xOf(s.a) + 1, x1 = xOf(s.b) - 1, c = s.taker ? teamColor(s.taker) : P.muted, chosen = state.t0 === s.a && state.t1 === s.b;
      g.globalAlpha = whole() || inSel(s.a, s.b) ? 1 : 0.35;
      g.beginPath(); g.roundRect(x0, 6, Math.max(2, x1 - x0), SECT, 6);
      g.fillStyle = P.raised; g.fill();
      g.lineWidth = chosen ? 2 : 1; g.strokeStyle = chosen ? P.select : c; g.stroke();
      g.font = `600 12.5px ${P.head}`; g.fillStyle = P.text; g.textAlign = 'left';
      const label = `${s.n}. ${s.name}`;
      let text = label;
      while (text.length > 2 && g.measureText(text).width > x1 - x0 - 14) text = text.slice(0, -2);
      if (text !== label) text = text.length > 3 ? text + '…' : `${s.n}`;
      g.fillText(text, x0 + 7, 6 + SECT / 2);
      g.globalAlpha = 1;
    }

    // kills
    const kt = 6 + top, mid = kt + KILLS / 2, amp = KILLS / 2 - 2, bw = Math.max(1, (w - LEFT - RIGHT) / n - 1);
    for (const tm of [1, 2]) {
      g.fillStyle = teamColor(tm);
      kills[tm].forEach((k, i) => {
        if (!k) return;
        const t = i * BUCKET, bh = Math.max(1.5, k / peak * amp);
        g.globalAlpha = inSel(t, t + BUCKET) ? 0.9 : 0.25;
        g.fillRect(xOf(t), tm === 1 ? mid - bh : mid + 1, bw, bh);
      });
    }
    g.globalAlpha = 1;
    g.fillStyle = P.line; g.fillRect(LEFT, mid, w - LEFT - RIGHT, 1);

    // axis
    const ay = height - AXIS / 2;
    const stepMin = [1, 2, 5, 10].find(m => (w - LEFT - RIGHT) / (END / 60 / m) >= 46) || 15;
    g.font = `12px ${P.body}`; g.fillStyle = P.muted;
    for (let t = 0; t <= END; t += stepMin * 60) {
      g.textAlign = t === 0 ? 'left' : 'center';
      g.fillText(clock(t), xOf(t), ay);
    }

    if (!whole()) {
      const x0 = xOf(state.t0), x1 = xOf(state.t1);
      g.fillStyle = P.dark ? 'rgba(255,255,255,.08)' : 'rgba(28,34,40,.07)';
      g.fillRect(x0, kt - 2, x1 - x0, KILLS + 4);
      g.fillStyle = P.text; g.fillRect(x0 - 1, kt - 2, 2, KILLS + 4); g.fillRect(x1 - 1, kt - 2, 2, KILLS + 4);
    }
    if (cursor != null) {
      const x = xOf(cursor), label = clock(cursor);
      g.font = `12px ${P.body}`;
      const lw = g.measureText(label).width + 10, lx = Math.min(Math.max(x - lw / 2, 2), w - lw - 2);
      g.fillStyle = P.text; g.globalAlpha = 0.5; g.fillRect(x, kt - 2, 1, KILLS + 4); g.globalAlpha = 1;
      g.beginPath(); g.roundRect(lx, height - AXIS + 2, lw, 17, 4); g.fill();
      g.fillStyle = P.panel; g.textAlign = 'left'; g.fillText(label, lx + 5, height - AXIS / 2 + 1);
    }
  }
  tc.addEventListener('pointerdown', e => {
    const s = sectorAt(e.offsetX, e.offsetY);
    if (s) {
      if (state.t0 === s.a && state.t1 === s.b) setRange(0, END);
      else { setRange(s.a, s.b); fitPoints(s.zones.map(z => z.pos), 300); }
      return;
    }
    dragFrom = tOf(e.offsetX); tc.setPointerCapture(e.pointerId);
  });
  tc.addEventListener('pointermove', e => {
    cursor = tOf(e.offsetX);
    tc.style.cursor = dragFrom == null && sectorAt(e.offsetX, e.offsetY) ? 'pointer' : 'crosshair';
    if (dragFrom != null) setRange(dragFrom, tOf(e.offsetX)); else draw();
  });
  tc.addEventListener('pointerup', () => { dragFrom = null; });
  tc.addEventListener('pointerleave', () => { cursor = null; draw(); });
  tc.addEventListener('dblclick', () => setRange(0, END));
  return { resize, render };
})();

function setRange(a, b) {
  state.t0 = Math.max(0, Math.min(a, b)); state.t1 = Math.min(END, Math.max(a, b));
  if (state.t1 - state.t0 < 1) { state.t0 = 0; state.t1 = END; }
  const sec = SECTORS.find(s => s.a === state.t0 && s.b === state.t1);
  $('stripHead').innerHTML = whole()
    ? `<b>${tr('wholeBattle')}</b>`
    : `<b>${sec ? tr('sectorSpan', sec.n, clock(sec.a), clock(sec.b)) : tr('timeRange', clock(state.t0), clock(state.t1))}</b>` +
      `<button class="link" id="rangeReset">${tr('showWhole')}</button>`;
  const reset = $('rangeReset');
  if (reset) reset.onclick = () => setRange(0, END);
  if (ready) renderPanel();
  draw();
}

// ---------- keyboard
document.addEventListener('keydown', e => {
  if ((e.target.closest && e.target.closest('input, textarea')) || e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key === 'Escape') { if (state.sel) select(null); else setRange(0, END); }
});

// ---------- layout
function resize() {
  DPR = window.devicePixelRatio || 1;
  timeline.resize();
  const r = cv.getBoundingClientRect();
  const first = !W;
  W = r.width; H = r.height;
  cv.width = W * DPR; cv.height = H * DPR;
  if (first) fitBattle();
  draw();
}

document.documentElement.lang = LANG_CODE;
let initialTheme = null;
try { initialTheme = localStorage.getItem(THEME_KEY); } catch (e) { /* storage may be unavailable */ }
applyTheme(initialTheme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'));
$('theme').onclick = () => applyTheme(document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark');
$('zoomIn').innerHTML = icon('plus'); $('zoomIn').title = tr('zoomIn');
$('zoomOut').innerHTML = icon('minus'); $('zoomOut').title = tr('zoomOut');
$('zoomFit').innerHTML = icon('locate-fixed'); $('zoomFit').title = tr('zoomFit');
renderTop(); renderPanel();
ready = true;
setRange(0, END);
window.addEventListener('resize', resize);
resize();
