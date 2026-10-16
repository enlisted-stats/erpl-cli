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
const teamName = n => team(n) ? team(n).armyName || capital(words(team(n).army || '')) || tr('team', n) : tr('team', n);
const playerName = i => i >= 0 ? D.players[i].name : tr('ai');
// Names come from the game data when it has them, otherwise from ids.
const vehicleName = v => v.name || capital(words(v.model.replace(/^(ussr|germ|ger|us|usa|uk|jp|jap|ita|it|fr|cn|builded)_/, '')));
const structureName = s => s.rally ? tr('rallyPoint') : s.name || capital(words(s.kind.replace(/^builded_/, '')));
const zoneName = z => z.label || capital(words(z.name || ''));
const weaponName = w => w >= 0 ? D.weapons[w].name : '';
const stat = (p, k) => num(p.stats[k]);

// ---------- tracks
// A track comes as differences dt, dx, dz with the time in tenths of a second; it is
// turned into t, x, z, t, x, z, ... with the time in seconds.
function decodeTrack(a) {
  const out = new Float64Array(a.length);
  let t = 0, x = 0, z = 0;
  for (let i = 0; i < a.length; i += 3) {
    t += a[i]; x += a[i + 1]; z += a[i + 2];
    out[i] = t / 10; out[i + 1] = x; out[i + 2] = z;
  }
  return out;
}
for (const s of D.soldiers) s.track = decodeTrack(s.track);
for (const v of D.vehicles) v.track = decodeTrack(v.track);

// posAt is the last track point at or before t, or the first one when t is before it.
function posAt(track, t) {
  if (!track.length) return null;
  let lo = 0, hi = track.length / 3 - 1;
  if (track[0] > t) return [track[1], track[2]];
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (track[mid * 3] <= t) lo = mid; else hi = mid - 1;
  }
  return [track[lo * 3 + 1], track[lo * 3 + 2]];
}
// trackLength is the length of a track on the ground, in meters.
function trackLength(track) {
  let d = 0;
  for (let i = 3; i < track.length; i += 3) d += Math.hypot(track[i + 1] - track[i - 2], track[i + 2] - track[i - 1]);
  return d;
}

// ---------- events
// GROUPS put the kinds of weapons into the groups the deaths can be filtered by.
const GROUPS = {
  small: ['pistol', 'submachine_gun', 'rifle', 'semiauto', 'assault_rifle', 'machine_gun', 'shotgun', 'melee'],
  explosive: ['grenade', 'molotov', 'tnt_block', 'mine', 'launcher', 'mortar', 'flamethrower'],
  vehicle: ['cannon', 'autocannon', 'vehicle_machine_gun', 'smoke'],
  air: ['aircraft_gun', 'bomb', 'rocket'],
  artillery: ['artillery'],
};
const GROUP_NAMES = { small: 'groupSmall', explosive: 'groupExplosive', vehicle: 'groupVehicle', air: 'groupAir', artillery: 'groupArtillery', other: 'groupOther' };
const groupOf = w => w >= 0 && Object.keys(GROUPS).find(g => GROUPS[g].includes(D.weapons[w].kind)) || 'other';

// KILLS are the deaths of soldiers and vehicles: where the victim was, who killed and
// from where.
const KILLS = [];
D.soldiers.forEach((s, i) => {
  if (!s.died) return;
  const p = posAt(s.track, s.died);
  if (p) KILLS.push({ t: s.died, soldier: i, team: s.team, player: s.player, by: s.killerSoldier, weapon: s.weapon, headshot: !!s.headshot, x: p[0], z: p[1] });
});
D.vehicles.forEach((v, i) => {
  if (!v.destroyed) return;
  const p = posAt(v.track, v.destroyed);
  if (p) KILLS.push({ t: v.destroyed, vehicle: i, team: v.team, player: v.player, by: v.killerSoldier, weapon: v.weapon, headshot: false, x: p[0], z: p[1] });
});
for (const k of KILLS) {
  const ks = k.by >= 0 ? D.soldiers[k.by] : null;
  k.byPlayer = ks ? ks.player : -1;
  k.byTeam = ks ? ks.team : 0;
  k.group = groupOf(k.weapon);
  const from = ks && posAt(ks.track, k.t);
  if (from && k.by !== k.soldier) { k.fx = from[0]; k.fz = from[1]; k.dist = Math.round(Math.hypot(k.x - from[0], k.z - from[1])); }
}
KILLS.sort((a, b) => a.t - b.t);
const enemyKill = k => k.by >= 0 && k.byTeam && k.byTeam !== k.team;

const DOWNS = [];
D.soldiers.forEach((s, i) => (s.downs || []).forEach(([t, by, revived]) => {
  const p = posAt(s.track, t);
  if (p) DOWNS.push({ t, soldier: i, team: s.team, player: s.player, by, revived, x: p[0], z: p[1] });
}));

// SPAWN_POINTS are the places squads came from: bases, rally points and APCs, each with
// its squads. Bases and rally points nobody used are there too.
const SPAWN_POINTS = (() => {
  const points = new Map();
  const point = (key, make) => { if (!points.has(key)) points.set(key, { ...make(), spawns: [] }); return points.get(key); };
  D.bases.forEach((b, i) => point('base' + i, () => ({ from: 'base', kind: b.kind, ref: i, team: b.team, player: -1, pos: b.pos, active: b.active })));
  D.structures.forEach((s, i) => { if (s.rally) point('rally' + i, () => ({ from: 'rally', ref: i, team: s.team, player: s.player, pos: s.pos, a: s.built, b: s.removed })); });
  D.spawns.forEach((sp, i) => {
    let p = null;
    if (sp.from === 'base' || sp.from === 'rally') p = points.get(sp.from + sp.ref);
    else if (sp.from === 'apc') {
      const v = D.vehicles[sp.ref];
      p = point('apc' + sp.ref, () => ({ from: 'apc', ref: sp.ref, team: v.team, player: v.player, pos: null, a: v.spawned, b: v.destroyed }));
    }
    if (p) p.spawns.push(i);
  });
  return [...points.values()];
})();
const spawnGlyph = p => p.from === 'base' ? (p.kind === 'infantry' ? 'flag' : 'tank') : p.from;
const BASE_NAMES = { infantry: 'baseInfantry', vehicles: 'baseVehicles', freeTank: 'baseFreeTank' };
const spawnPointName = p => p.from === 'rally' ? tr('rallyPoint') : p.from === 'apc' ? tr('apcRespawn', vehicleName(D.vehicles[p.ref])) : tr(BASE_NAMES[p.kind]);
// spawnPointOpen tells whether a place squads came from was there within the chosen time.
// A base must be open a little within the time: at the end of a sector the bases of the
// next one open in the same second.
const spawnPointOpen = p => p.from === 'base' ? p.active.some(([a, b]) => Math.min(b, state.t1) - Math.max(a, state.t0) > Math.min(2, (state.t1 - state.t0) / 4))
  : p.from === 'apc' ? pointSpawns(p).length > 0 : overlaps(p.a, p.b);

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
  if (ready) { renderTop(); renderPanel(); renderCard(); }
  draw();
}

// ---------- state
// layers are what the map shows; groups and kinds filter the deaths and the vehicles.
// sel is the object the card is about: {type, id} with the type player, vehicle,
// structure, spawn (an index in SPAWN_POINTS), kill (in KILLS) or down (in DOWNS).
const state = {
  team: 0, t0: 0, t1: END, sel: null, hot: null, list: 'players', skipSpawn: true,
  layers: { heat: true, deaths: false, downs: false, spawns: true, vehicles: true, structures: false },
  groups: new Set(Object.keys(GROUP_NAMES)),
  kinds: new Set(['tank', 'apc', 'bike', 'car', 'gun', 'ship']),
};
const same = (a, b) => !!(a && b && a.type === b.type && a.id === b.id);
const ofTeam = t => !state.team || t === state.team;
const whole = () => state.t0 <= 0 && state.t1 >= END;
const inTime = t => t >= state.t0 && t <= state.t1;
const overlaps = (a, b) => a <= state.t1 && (b || END) >= state.t0;

// ---------- time and sectors
// zoneActive tells whether the zone was active for a while within the chosen time; zones
// without activity data are active all the time.
const zoneActive = z => !z.active || z.active.some(([a, b]) => a < state.t1 && b > state.t0);
// ownerAt is the owner after the last change at or before t: a team, -1 for nobody, or undefined.
function ownerAt(z, t) {
  let o;
  for (const [ct, tm] of z.owners || []) if (ct <= t) o = tm;
  return o;
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
    return { group, zones, a, b, taker: after > 0 && after !== before ? after : 0 };
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
// The world is drawn in meters: x to the right, z to the top of the screen.
let L, T, R, B;
if (D.map) {
  [L, T] = D.map.leftTop;
  [R, B] = D.map.rightBottom;
} else {
  const b = box(D.soldiers, 0.01, 0.99);
  const side = Math.max(b.r - b.l, b.t - b.b, 200) * 1.3, cx = (b.l + b.r) / 2, cz = (b.t + b.b) / 2;
  [L, T, R, B] = [cx - side / 2, cz + side / 2, cx + side / 2, cz - side / 2];
}
const MAX_ZOOM = 30;

// box is the box around the track points of the soldiers between two quantiles.
function box(soldiers, qlo, qhi) {
  const xs = [], zs = [];
  for (const s of soldiers) for (let j = 0; j < s.track.length; j += 3) { xs.push(s.track[j + 1]); zs.push(s.track[j + 2]); }
  if (!xs.length) return { l: -100, r: 100, t: 100, b: -100 };
  xs.sort((a, b) => a - b); zs.sort((a, b) => a - b);
  const q = (a, f) => a[Math.min(a.length - 1, Math.floor(f * (a.length - 1)))];
  return { l: q(xs, qlo), r: q(xs, qhi), b: q(zs, qlo), t: q(zs, qhi) };
}

const cv = $('map'), ctx = cv.getContext('2d');
const view = { k: 1, ox: 0, oy: 0 }; // pixels per meter, screen position of the world point (L, T)
let W = 0, H = 0, DPR = 1;
const sx = x => (x - L) * view.k + view.ox;
const sy = z => (T - z) * view.k + view.oy;

function fitWorld(b, pad = 1.2) {
  const bw = Math.max(b.r - b.l, 30) * pad, bh = Math.max(b.t - b.b, 30) * pad;
  view.k = Math.min(MAX_ZOOM, Math.max(0.05, Math.min(W / bw, H / bh)));
  view.ox = W / 2 - ((b.l + b.r) / 2 - L) * view.k;
  view.oy = H / 2 - (T - (b.t + b.b) / 2) * view.k;
  draw();
}
const fitBattle = () => fitWorld(box(D.soldiers, 0.02, 0.98));
function fitPoints(pts, minSide = 150) {
  if (!pts.length) return;
  const xs = pts.map(p => p[0]), zs = pts.map(p => p[1]);
  const cx = (Math.min(...xs) + Math.max(...xs)) / 2, cz = (Math.min(...zs) + Math.max(...zs)) / 2;
  const hw = Math.max(Math.max(...xs) - Math.min(...xs), minSide) / 2, hh = Math.max(Math.max(...zs) - Math.min(...zs), minSide) / 2;
  fitWorld({ l: cx - hw, r: cx + hw, b: cz - hh, t: cz + hh });
}

// ---------- selection
// related tells what the selection is about, so the rest of the map can step back:
// soldiers, vehicles, spawns and kills of a player, the crew and the kills of a vehicle.
let REL = null;
function related() {
  const sel = state.sel;
  if (!sel) return null;
  const r = { soldiers: new Set(), vehicles: new Set(), spawns: new Set(), kills: new Set(), structures: new Set(), points: new Set(), downs: new Set() };
  if (sel.type === 'player') {
    D.soldiers.forEach((s, i) => { if (s.player === sel.id) r.soldiers.add(i); });
    D.vehicles.forEach((v, i) => { if (v.player === sel.id) r.vehicles.add(i); });
    D.structures.forEach((s, i) => { if (s.player === sel.id) r.structures.add(i); });
    D.spawns.forEach((s, i) => { if (s.player === sel.id) r.spawns.add(i); });
    KILLS.forEach((k, i) => { if (k.player === sel.id || k.byPlayer === sel.id) r.kills.add(i); });
  } else if (sel.type === 'vehicle') {
    const v = D.vehicles[sel.id];
    r.vehicles.add(sel.id);
    v.crew.forEach(i => r.soldiers.add(i));
    KILLS.forEach((k, i) => {
      if (k.vehicle === sel.id) r.kills.add(i);
      // a kill by the crew while the vehicle went on, with a weapon of a vehicle or a plane
      else if (v.crew.includes(k.by) && k.t >= v.spawned && k.t <= (v.destroyed || END) && (k.group === 'vehicle' || k.group === 'air')) r.kills.add(i);
    });
    if (v.structure >= 0) r.structures.add(v.structure);
  } else if (sel.type === 'spawn') {
    const p = SPAWN_POINTS[sel.id];
    r.points.add(sel.id);
    p.spawns.forEach(i => r.spawns.add(i));
    if (p.from === 'apc') r.vehicles.add(p.ref);
  } else if (sel.type === 'structure') {
    r.structures.add(sel.id);
  } else if (sel.type === 'kill') {
    r.kills.add(sel.id);
  } else if (sel.type === 'down') {
    r.downs.add(sel.id);
  }
  return r;
}

// ---------- heatmap
// The grid covers where almost all soldiers were (pilots fly far away) and has at most
// about 600 cells a side, so it is rebuilt at once when a filter changes.
const heat = { key: '', canvas: document.createElement('canvas') };
(function heatGrid() {
  const b = box(D.soldiers, 0.01, 0.99), pad = 150;
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
  // the heat of a player when one is chosen
  const only = REL && state.sel.type === 'player' ? REL.soldiers : null;
  D.soldiers.forEach((s, si) => {
    if (!ofTeam(s.team) || (only && !only.has(si))) return;
    const tr_ = s.track;
    for (let i = 0; i < tr_.length; i += 3) {
      const t = tr_[i];
      if (t < state.t0 || t > state.t1 || (state.skipSpawn && t - s.spawned < 10)) continue;
      const cx = Math.floor((tr_[i + 1] - heat.l) / cell), cy = Math.floor((heat.t - tr_[i + 2]) / cell);
      if (cx >= 0 && cy >= 0 && cx < w && cy < h) g[cy * w + cx]++;
    }
  });
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
    img.data.set([r, gg, b, Math.round(255 * Math.min(0.75, 0.08 + v))], i * 4);
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
// A marker is a symbol on a disc of the team color. Something destroyed is a dark disc
// with a ring of the team color and a flash. The same SVG is used on the map, in the
// panel and in the card, so they look alike.
function markerSVG(name, color, px, dead, square) {
  const shape = square ? '<rect x="1.6" y="1.6" width="28.8" height="28.8" rx="6"' : '<circle cx="16" cy="16" r="14.5"';
  const disc = dead
    ? `${shape} fill="#262b31" stroke="${color}" stroke-width="2.6"/>`
    : `${shape} fill="${color}" stroke="#ffffff" stroke-width="2"/>`;
  const flash = dead ? `<circle cx="25.5" cy="25.5" r="6.2" fill="#262b31"/>` +
    `<g transform="translate(18.6 18.6) scale(.027)" fill="#ffa53a">${GLYPHS.burst}</g>` : '';
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="-1 -1 34 34">${disc}` +
    `<g transform="translate(5 5) scale(.043)" fill="${dead ? '#c9ced3' : '#ffffff'}">${GLYPHS[name]}</g>${flash}</svg>`;
}
const markerURL = (name, color, px, dead, square) => 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(markerSVG(name, color, px, dead, square));
const markerImg = (name, color, dead, square) => `<img class="mk" src="${markerURL(name, color, 40, dead, square)}" alt="">`;
const sprites = new Map();
function marker(name, color, size, x, y, alpha = 1, dead = false, square = false) {
  const key = `${name}|${color}|${size}|${DPR}|${dead}|${square}`;
  let img = sprites.get(key);
  if (!img) {
    img = new Image();
    img.onload = draw;
    img.src = markerURL(name, color, Math.round(size * DPR), dead, square);
    sprites.set(key, img);
  }
  if (!img.complete || !img.naturalWidth) return;
  ctx.globalAlpha = alpha;
  ctx.drawImage(img, Math.round(x - size / 2), Math.round(y - size / 2), size, size);
  ctx.globalAlpha = 1;
}
// badge draws a number in a pill at the top right of a marker.
function badge(n, x, y, size, alpha) {
  const text = String(n);
  ctx.font = `700 11px ${P.body}`;
  const w = Math.max(16, ctx.measureText(text).width + 8), h = 15, bx = x + size * 0.28, by = y - size * 0.62;
  ctx.globalAlpha = alpha;
  ctx.beginPath(); ctx.roundRect(bx, by, w, h, h / 2);
  ctx.fillStyle = '#ffffff'; ctx.fill();
  ctx.lineWidth = 1; ctx.strokeStyle = 'rgba(15,17,19,.45)'; ctx.stroke();
  ctx.fillStyle = '#1c2228'; ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
  ctx.fillText(text, bx + w / 2, by + h / 2 + 0.5);
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
  if (casing) { ctx.strokeStyle = casing; ctx.lineWidth = width + 2; ctx.stroke(); }
  ctx.strokeStyle = color; ctx.lineWidth = width; ctx.stroke();
  ctx.globalAlpha = 1;
}
function dot(x, y, r, fill, line, alpha = 1) {
  ctx.globalAlpha = alpha;
  ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2);
  if (fill) { ctx.fillStyle = fill; ctx.fill(); }
  if (line) { ctx.lineWidth = 1.4; ctx.strokeStyle = line; ctx.stroke(); }
  ctx.globalAlpha = 1;
}
// line draws a kill line from where the killer was to the victim, with an arrowhead.
function killLine(k, alpha) {
  if (k.fx == null) return;
  const x0 = sx(k.fx), y0 = sy(k.fz), x1 = sx(k.x), y1 = sy(k.z), a = Math.atan2(y1 - y0, x1 - x0);
  ctx.beginPath(); ctx.moveTo(x0, y0); ctx.lineTo(x1, y1);
  stroke(teamColor(k.byTeam), 1.6, alpha, 'rgba(15,17,19,.4)');
  ctx.globalAlpha = alpha;
  ctx.beginPath(); ctx.moveTo(x1, y1);
  ctx.lineTo(x1 - 8 * Math.cos(a - 0.4), y1 - 8 * Math.sin(a - 0.4));
  ctx.lineTo(x1 - 8 * Math.cos(a + 0.4), y1 - 8 * Math.sin(a + 0.4));
  ctx.closePath(); ctx.fillStyle = teamColor(k.byTeam); ctx.fill();
  dot(x0, y0, 3, '#ffffff', teamColor(k.byTeam), alpha);
  ctx.globalAlpha = 1;
}

// hits are the things drawn in the last frame that the pointer can pick, topmost last.
let hits = [];
const hit = (x, y, r, obj, tipHTML) => hits.push({ x, y, r, obj, tip: tipHTML });

// shown* are the objects of the layers within the chosen team and time.
const shownVehicles = () => D.vehicles.map((v, i) => [v, i]).filter(([v]) =>
  v.track.length && v.player >= 0 && ofTeam(v.team) && state.kinds.has(v.kind) && overlaps(v.spawned, v.destroyed));
const shownStructures = () => D.structures.map((s, i) => [s, i]).filter(([s]) => !s.rally && ofTeam(s.team) && overlaps(s.built, s.removed));
const shownKills = () => KILLS.map((k, i) => [k, i]).filter(([k]) => inTime(k.t) && ofTeam(k.team) && state.groups.has(k.group));
const shownDowns = () => DOWNS.map((d, i) => [d, i]).filter(([d]) => inTime(d.t) && ofTeam(d.team));
const pointSpawns = p => p.spawns.filter(i => inTime(D.spawns[i].t) && ofTeam(D.spawns[i].team));
const shownSpawnPoints = () => SPAWN_POINTS.map((p, i) => [p, i]).filter(([p]) => ofTeam(p.team) && spawnPointOpen(p));

function render() {
  frame = 0;
  REL = related();
  hits = [];
  ctx.setTransform(DPR, 0, 0, DPR, 0, 0);
  ctx.fillStyle = P.mapbg;
  ctx.fillRect(0, 0, W, H);
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = 'high';
  if (D.map && mapImg.complete && mapImg.naturalWidth) {
    const x0 = sx(L), y0 = sy(T), w = (R - L) * view.k, h = (T - B) * view.k;
    ctx.drawImage(mapImg, x0, y0, w, h);
    if (P.dark) { ctx.fillStyle = 'rgba(15,17,19,.3)'; ctx.fillRect(x0, y0, w, h); }
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
    ctx.fillStyle = color; ctx.globalAlpha = 0.14; ctx.fill(); ctx.globalAlpha = 1;
    stroke(color, 1.5);
  }

  if (state.layers.heat) drawHeat();
  if (state.layers.structures) drawStructures();
  if (state.layers.vehicles) drawVehicles();
  if (state.layers.spawns) drawSpawns();
  if (state.layers.downs) drawDowns();
  drawKills();

  for (const z of zones) zoneLabel(z);
  scaleBar();
  timeline.render();
}

const faded = (rel, set, i) => !REL ? 1 : REL[set].has(i) ? 1 : rel;

function drawHeat() {
  const key = [state.team, state.skipSpawn, state.t0, state.t1, state.sel && state.sel.type === 'player' ? state.sel.id : ''].join('|');
  if (key !== heat.key) { buildHeat(); heat.key = key; }
  const s = heat.cell * view.k;
  ctx.drawImage(heat.canvas, sx(heat.l), sy(heat.t), heat.w * s, heat.h * s);
}

function drawStructures() {
  for (const [s, i] of shownStructures()) {
    const a = faded(0.3, 'structures', i), dead = s.removed && s.removed <= state.t1;
    const x = sx(s.pos[0]), y = sy(s.pos[1]), size = 18;
    marker(glyphOfStructure(s.kind), teamColor(s.team), size, x, y, a, dead);
    hit(x, y, size / 2 + 2, { type: 'structure', id: i }, `<b>${esc(structureName(s))}</b><div>${tr('builtBy', colored(playerName(s.player), s.team), clock(s.built))}</div>`);
  }
}

// vehicleRoute strokes the part of a route within the chosen time.
function vehicleRoute(v) {
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
const vehicleGlyph = v => ({ tank: 'tank', apc: 'apc', plane: 'plane', bike: 'bike', gun: 'gunAT', ship: 'boat' })[v.kind] || 'car';

function drawVehicles() {
  const vs = shownVehicles();
  for (const [v, i] of vs) {
    const plane = v.kind === 'plane', a = faded(0.2, 'vehicles', i);
    const on = REL && REL.vehicles.has(i);
    vehicleRoute(v);
    if (on || same(state.hot, { type: 'vehicle', id: i })) stroke(P.select, (plane ? 1 : 2) + 3.5, 0.85, null);
    stroke(teamColor(v.team), plane ? 1 : 2, a * (plane ? 0.7 : 0.95));
  }
  for (const [v, i] of vs) {
    const a = faded(0.25, 'vehicles', i);
    const dead = v.destroyed && v.destroyed <= state.t1;
    const p = posAt(v.track, dead ? v.destroyed : Math.min(state.t1, v.destroyed || END));
    if (!p) continue;
    const x = sx(p[0]), y = sy(p[1]), size = 22;
    marker(vehicleGlyph(v), teamColor(v.team), size, x, y, a, dead);
    hit(x, y, size / 2 + 2, { type: 'vehicle', id: i }, vehicleTip(v));
  }
}
function vehicleTip(v) {
  return `<b>${esc(vehicleName(v))}</b><div>${colored(playerName(v.player), v.team)}</div>` +
    `<div class="m">${v.destroyed ? tr('knockedOutAt', clock(v.destroyed)) : tr('survivedBattle')}</div>`;
}

function drawSpawns() {
  for (const [p, i] of shownSpawnPoints()) {
    const n = pointSpawns(p), a = faded(0.3, 'points', i), color = teamColor(p.team);
    if (p.from === 'apc') {
      // an APC moves: a mark at each place squads left it
      for (const si of n) {
        const s = D.spawns[si], x = sx(s.pos[0]), y = sy(s.pos[1]);
        marker('apc', color, 18, x, y, a);
        hit(x, y, 11, { type: 'spawn', id: i }, spawnTip(p, n.length));
      }
      continue;
    }
    const dead = p.b && p.b <= state.t1;
    // a base that closed before the end of the chosen time steps back, as the zones show
    // who held them at the end
    const closed = p.from === 'base' && !p.active.some(([s, e]) => s <= state.t1 && e >= state.t1 - 1);
    const x = sx(p.pos[0]), y = sy(p.pos[1]), size = p.from === 'rally' ? 24 : closed ? 20 : 26, alpha = closed ? a * 0.5 : a;
    marker(spawnGlyph(p), color, size, x, y, alpha, dead && p.from === 'rally', p.from === 'base');
    if (n.length) badge(n.length, x, y, size, alpha);
    hit(x, y, size / 2 + 2, { type: 'spawn', id: i }, spawnTip(p, n.length));
  }
  if (REL && REL.spawns.size) for (const i of REL.spawns) {
    const s = D.spawns[i];
    if (!inTime(s.t)) continue;
    dot(sx(s.pos[0]), sy(s.pos[1]), 3.2, teamColor(s.team), '#ffffff');
  }
}
const spawnTip = (p, n) => `<b>${esc(spawnPointName(p))}</b><div>${tr('squadsHere', n)}</div>` +
  (p.player >= 0 ? `<div class="m">${colored(playerName(p.player), p.team)}</div>` : '');

function drawDowns() {
  for (const [d, i] of shownDowns()) {
    const a = REL ? (REL.downs.has(i) || REL.soldiers.has(d.soldier) || REL.soldiers.has(d.by) ? 1 : 0.2) : 1;
    const x = sx(d.x), y = sy(d.z);
    if (REL && REL.downs.has(i)) marker('downed', teamColor(d.team), 20, x, y);
    dot(x, y, 3.6, d.revived ? '#ffffff' : null, teamColor(d.team), a);
    dot(x, y, 2.2, null, teamColor(d.team), a);
    hit(x, y, 6, { type: 'down', id: i }, downTip(d));
  }
}
function downTip(d) {
  const by = d.by >= 0 ? D.soldiers[d.by] : null;
  return `<b>${colored(playerName(d.player), d.team)}</b>` +
    (by ? `<div>${tr('downedBy', colored(playerName(by.player), by.team))}</div>` : '') + `<div>${tr('timeAt', clock(d.t))}</div>` +
    `<div class="m">${d.revived ? tr('revived', clock(d.revived)) : tr('notRevived')}</div>`;
}

// drawKills draws the deaths layer, and the kills of the selection as lines from the
// killer to the victim even when the layer is off.
function drawKills() {
  const ks = state.layers.deaths ? shownKills() : [];
  for (const [k, i] of ks) {
    if (k.vehicle != null) continue; // vehicles show where they were knocked out themselves
    const a = REL ? (REL.kills.has(i) ? 1 : 0.18) : 0.9, x = sx(k.x), y = sy(k.z);
    dot(x, y, k.headshot ? 3.4 : 2.8, teamColor(k.team), '#ffffff', a);
    hit(x, y, 6, { type: 'kill', id: i }, killTip(k));
  }
  if (!REL) return;
  for (const i of REL.kills) {
    const k = KILLS[i];
    if (!inTime(k.t) || !ofTeam(k.team) && !ofTeam(k.byTeam)) continue;
    killLine(k, 0.95);
    if (k.vehicle == null) {
      const x = sx(k.x), y = sy(k.z);
      marker('skull', teamColor(k.team), 20, x, y);
      hit(x, y, 11, { type: 'kill', id: i }, killTip(k));
    }
  }
}
function killTip(k) {
  const victim = k.vehicle != null ? `${vehicleName(D.vehicles[k.vehicle])} (${playerName(k.player)})` : playerName(k.player);
  let head;
  if (k.by < 0) head = `<b>${colored(victim, k.team)}</b><div>${tr('notBySoldier')}</div>`;
  else if (k.by === k.soldier) head = `<b>${colored(victim, k.team)}</b><div>${tr('suicide')}</div>`;
  else head = `<b>${tr('killedBy', colored(playerName(k.byPlayer), k.byTeam), colored(victim, k.team))}</b>`;
  const rows = [weaponName(k.weapon) && tr('weapon', esc(weaponName(k.weapon))), k.dist != null && tr('shotFrom', k.dist), k.headshot && tr('headshotRow'), tr('timeAt', clock(k.t))];
  return head + rows.filter(Boolean).map(r => `<div>${r}</div>`).join('');
}

function grid() {
  const step = 100 * view.k;
  ctx.strokeStyle = P.line; ctx.lineWidth = 1;
  ctx.beginPath();
  const x0 = sx(L), y0 = sy(T), w = (R - L) * view.k, h = (T - B) * view.k;
  for (let v = 0; v <= w + 0.5; v += step) { ctx.moveTo(x0 + v, y0); ctx.lineTo(x0 + v, y0 + h); }
  for (let v = 0; v <= h + 0.5; v += step) { ctx.moveTo(x0, y0 + v); ctx.lineTo(x0 + w, y0 + v); }
  ctx.stroke();
}

function zoneShape(z) {
  if (z.area && z.area.length > 2) path(z.area);
  else { ctx.beginPath(); ctx.arc(sx(z.pos[0]), sy(z.pos[1]), Math.max(z.radius || 10, 4) * view.k, 0, Math.PI * 2); }
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
  hit(x, y, Math.max(w, h) / 2, null, zoneTip(z));
}
function zoneTip(z) {
  const sec = SECTORS.find(s => s.zones.includes(z));
  const owners = (z.owners || []).map(([t, o]) => `${clock(t)} ${o > 0 ? colored(teamName(o), o) : tr('nobody')}`).join(', ');
  return `<b>${esc(z.title ? `${z.title}, ${zoneName(z)}` : zoneName(z))}</b>` +
    (sec ? `<div>${tr('sectorSpan', sec.n, clock(sec.a), clock(sec.b))}</div>` : '') +
    (owners ? `<div class="m">${tr('owners')}: ${owners}</div>` : '');
}

function scaleBar() {
  const target = 110 / view.k;
  const m = [2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000].find(v => v >= target * 0.6) || 5000;
  $('scale').innerHTML = `<i style="width:${m * view.k}px"></i>${tr('meters', m)}`;
}

// ---------- map pointer
function zoomAt(f, px, py) {
  const k = Math.min(MAX_ZOOM, Math.max(0.05, view.k * f));
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
    const h = pick(e.offsetX, e.offsetY) || routeAt(e.offsetX, e.offsetY);
    if (h && h.obj) select(h.obj, false);
    else if (!h) select(null);
  }
  drag = null; cv.classList.remove('drag');
});
cv.addEventListener('pointerleave', () => { $('tip').style.display = 'none'; });
$('zoomIn').onclick = () => zoomAt(1.5, W / 2, H / 2);
$('zoomOut').onclick = () => zoomAt(1 / 1.5, W / 2, H / 2);
$('zoomFit').onclick = fitBattle;

const colored = (text, t) => `<span style="color:${teamColor(t)}">${esc(text)}</span>`;
// pick finds the topmost thing drawn under the pointer.
function pick(px, py) {
  for (let i = hits.length - 1; i >= 0; i--) if (Math.hypot(hits[i].x - px, hits[i].y - py) <= hits[i].r) return hits[i];
  return null;
}
// routeAt finds a vehicle route near the pointer.
function routeAt(px, py) {
  if (!state.layers.vehicles) return null;
  let best = 7, found = null;
  for (const [v, i] of shownVehicles()) {
    const tr_ = v.track;
    for (let j = 0; j < tr_.length; j += 3) {
      if (tr_[j] < state.t0 || tr_[j] > state.t1) continue;
      const d = Math.hypot(sx(tr_[j + 1]) - px, sy(tr_[j + 2]) - py);
      if (d < best) { best = d; found = { obj: { type: 'vehicle', id: i }, tip: vehicleTip(v) + `<div class="m">${tr('timeAt', clock(tr_[j]))}</div>` }; }
    }
  }
  return found;
}

function hover(px, py) {
  const h = pick(px, py) || routeAt(px, py);
  cv.classList.toggle('point', !!(h && h.obj));
  const hot = h && h.obj && h.obj.type === 'vehicle' ? h.obj : null;
  if (!same(hot, state.hot) && (hot || state.hot)) { state.hot = hot; draw(); }
  const tip = $('tip');
  if (!h || !h.tip) { tip.style.display = 'none'; return; }
  tip.innerHTML = h.tip;
  tip.style.display = 'block';
  tip.style.left = Math.min(px + 16, W - tip.offsetWidth - 8) + 'px';
  tip.style.top = Math.min(py + 16, H - tip.offsetHeight - 8) + 'px';
}

// select makes obj the selection, or clears it when it is already selected. fit moves
// the map to it.
function select(obj, fit) {
  state.sel = obj && !same(obj, state.sel) ? obj : null;
  REL = related();
  renderCard(); renderLists();
  if (fit && state.sel) {
    const pts = selectionPoints();
    if (pts.length) fitPoints(pts, 120);
  }
  draw();
}
function selectionPoints() {
  const sel = state.sel, pts = [];
  const trackPts = tr_ => { for (let i = 0; i < tr_.length; i += 3) if (inTime(tr_[i])) pts.push([tr_[i + 1], tr_[i + 2]]); };
  if (sel.type === 'vehicle') trackPts(D.vehicles[sel.id].track);
  else if (sel.type === 'structure') pts.push(D.structures[sel.id].pos);
  else if (sel.type === 'spawn') {
    const p = SPAWN_POINTS[sel.id];
    if (p.pos) pts.push(p.pos);
    p.spawns.forEach(i => pts.push(D.spawns[i].pos));
  } else if (sel.type === 'kill' || sel.type === 'down') {
    const k = (sel.type === 'kill' ? KILLS : DOWNS)[sel.id];
    pts.push([k.x, k.z]);
    if (k.fx != null) pts.push([k.fx, k.fz]);
  } else if (sel.type === 'player') {
    KILLS.forEach(k => { if (inTime(k.t) && (k.player === sel.id || k.byPlayer === sel.id)) pts.push([k.x, k.z]); });
  }
  return pts;
}

// ---------- card of the selection
function renderCard() {
  const card = $('card'), sel = state.sel;
  if (!sel) { card.hidden = true; return; }
  let sym = '', title = '', sub = '', rows = [];
  const row = (text, cls = '') => rows.push(`<p class="${cls}">${text}</p>`);
  if (sel.type === 'player') {
    const p = D.players[sel.id];
    const mine = KILLS.filter(k => k.byPlayer === sel.id && enemyKill(k));
    title = esc(p.name);
    sub = [colored(teamName(p.team), p.team), p.platoon && tr('platoon', p.platoon), p.bot && tr('bot'), p.left && tr('leftAt', clock(p.left))].filter(Boolean).join(' · ');
    const stats = [
      [tr('statKills'), mine.filter(k => k.vehicle == null).length], [tr('statDeaths'), stat(p, 'soldierDeaths')],
      [tr('statHeadshots'), mine.filter(k => k.headshot).length], [tr('statVehicles'), mine.filter(k => k.vehicle != null).length],
      [tr('statScore'), stat(p, 'score')],
    ];
    rows.push(`<dl class="stats">${stats.map(([k, v]) => `<div><dt>${k}</dt><dd>${v}</dd></div>`).join('')}</dl>`);
    sym = `<span class="avatar" style="--c:${teamColor(p.team)}">${esc(p.name.charAt(0).toUpperCase())}</span>`;
  } else if (sel.type === 'vehicle') {
    const v = D.vehicles[sel.id];
    sym = markerImg(vehicleGlyph(v), teamColor(v.team), !!v.destroyed);
    title = esc(vehicleName(v));
    sub = tr('kindOne_' + v.kind);
    if (v.structure >= 0) row(tr('builtBy', colored(playerName(v.player), v.team), clock(v.spawned)));
    else { row(tr('player', colored(playerName(v.player), v.team))); row(tr('spawnedAt', clock(v.spawned))); }
    if (v.destroyed) {
      const by = v.killerSoldier >= 0 ? D.soldiers[v.killerSoldier] : null;
      row(by ? tr('knockedOutBy', colored(playerName(by.player), by.team), clock(v.destroyed)) : tr('knockedOutAt', clock(v.destroyed)));
      if (v.weapon >= 0) row(tr('weapon', esc(weaponName(v.weapon))), 'm');
    } else row(tr('survivedBattle'));
    if (v.crew.length) row(tr('crew', v.crew.length), 'm');
    const km = trackLength(v.track) / 1000;
    if (km >= 0.05) row(tr('distance', km.toFixed(1)), 'm');
  } else if (sel.type === 'structure') {
    const s = D.structures[sel.id];
    sym = markerImg(glyphOfStructure(s.kind), teamColor(s.team), !!s.removed);
    title = esc(structureName(s));
    row(tr('builtBy', colored(playerName(s.player), s.team), clock(s.built)));
    row(s.removed ? tr('destroyedAt', clock(s.removed)) : tr('standsToEnd'), 'm');
  } else if (sel.type === 'spawn') {
    const p = SPAWN_POINTS[sel.id], n = pointSpawns(p);
    sym = markerImg(spawnGlyph(p), teamColor(p.team), p.from === 'rally' && !!p.b, p.from === 'base');
    title = esc(spawnPointName(p));
    sub = colored(teamName(p.team), p.team);
    if (p.from === 'base') row(tr('baseOpen', p.active.map(([a, b]) => tr('timeRange', clock(a), clock(b))).join(', ')), 'm');
    if (p.from === 'rally') {
      const s = D.structures[p.ref];
      row(tr('builtBy', colored(playerName(s.player), s.team), clock(s.built)));
      row(s.removed ? tr('destroyedAt', clock(s.removed)) : tr('standsToEnd'), 'm');
    }
    row(`<b>${tr('squadsHere', n.length)}</b>`);
    const by = new Map();
    n.forEach(i => by.set(D.spawns[i].player, (by.get(D.spawns[i].player) || 0) + 1));
    if (by.size) rows.push(`<ul class="who">${[...by].sort((a, b) => b[1] - a[1]).map(([pl, c]) =>
      `<li><button class="link" data-player="${pl}">${esc(playerName(pl))}</button><span>${c}</span></li>`).join('')}</ul>`);
  } else if (sel.type === 'kill') {
    const k = KILLS[sel.id];
    sym = k.vehicle != null ? markerImg(vehicleGlyph(D.vehicles[k.vehicle]), teamColor(k.team), true) : markerImg('skull', teamColor(k.team));
    rows.push(killTip(k));
  } else if (sel.type === 'down') {
    sym = markerImg('downed', teamColor(DOWNS[sel.id].team));
    rows.push(downTip(DOWNS[sel.id]));
  }
  card.innerHTML = `<header>${sym}<div class="h">${title ? `<h2>${title}</h2>` : ''}${sub ? `<p class="cardsub">${sub}</p>` : ''}</div>` +
    `<button class="iconbtn close" title="${tr('close')}">${icon('x')}</button></header><div class="body">${rows.join('')}</div>`;
  card.hidden = false;
}
$('card').addEventListener('click', e => {
  if (e.target.closest('.close')) return select(null);
  const pl = e.target.closest('[data-player]');
  if (pl && +pl.dataset.player >= 0) select({ type: 'player', id: +pl.dataset.player }, true);
});

// ---------- header
function renderTop() {
  const m = D.meta;
  $('mission').textContent = m.missionName || capital(words(m.mission));
  const date = m.started ? new Date(m.started).toLocaleString(LANG_CODE, { dateStyle: 'long', timeStyle: 'short' }) : '';
  const mode = [m.modeName || capital(words(m.mode || '')), m.campaignName || capital(words(m.campaign || ''))].filter(Boolean).join(', ');
  const minutes = Math.floor(m.duration / 60), seconds = Math.floor(m.duration % 60);
  $('subtitle').textContent = [mode, date, tr('lasted', tr('minutesSeconds', minutes, seconds))].filter(Boolean).join('. ') + '.';
  $('result').innerHTML = m.winner
    ? `<span class="won" style="--c:${teamColor(m.winner)}">${icon('trophy')}${esc(tr('won', teamName(m.winner)))}</span>`
    : `<span class="won">${esc(tr('noWinner'))}</span>`;
  $('teams').innerHTML = D.teams.map(t => {
    const k = KILLS.filter(x => x.byTeam === t.number && x.vehicle == null && enemyKill(x)).length, n = D.players.filter(p => p.team === t.number).length;
    return `<div class="teamchip" style="--c:${teamColor(t.number)}"><b>${esc(teamName(t.number))}</b>` +
      `<span>${tr('nPlayers', n)}, ${k} ${tr('killsWord', k)}</span></div>`;
  }).join('');
}

// ---------- panel
const LAYERS = [
  ['heat', null, 'layerHeat', 'layerHeatHint'],
  ['spawns', 'rally', 'layerSpawns', 'layerSpawnsHint'],
  ['vehicles', 'tank', 'layerVehicles'],
  ['deaths', 'skull', 'layerDeaths'],
  ['downs', 'downed', 'layerDowns'],
  ['structures', 'wall', 'layerStructures', 'layerStructuresHint'],
];
const KINDS = ['tank', 'apc', 'plane', 'bike', 'car', 'gun', 'ship'].filter(k => D.vehicles.some(v => v.kind === k && v.player >= 0));
function layerCount(id) {
  switch (id) {
    case 'deaths': return shownKills().filter(([k]) => k.vehicle == null).length;
    case 'downs': return shownDowns().length;
    case 'spawns': return D.spawns.filter(s => inTime(s.t) && ofTeam(s.team) && s.from && s.from !== 'vehicle' && s.from !== 'air').length;
    case 'vehicles': return shownVehicles().length;
    case 'structures': return shownStructures().length;
  }
  return '';
}

function renderPanel() {
  $('filters').innerHTML = `<div class="seg" id="teamSeg">` +
    [[0, tr('bothTeams'), null], ...D.teams.map(t => [t.number, teamName(t.number), teamColor(t.number)])]
      .map(([v, l, c]) => `<button data-v="${v}" aria-pressed="${v === state.team}">${c ? `<i style="--c:${c}"></i>` : ''}${esc(l)}</button>`).join('') + '</div>';
  renderLayers();
  renderLists();
}

function renderLayers() {
  const c = teamColor(state.team || 1);
  const chips = (set, items, name) => `<div class="chips">${items.map(id =>
    `<button data-${set}="${id}" aria-pressed="${state[set].has(id)}">${esc(tr(name(id)))}</button>`).join('')}</div>`;
  $('layers').innerHTML = `<h3>${tr('layers')}</h3>` + LAYERS.map(([id, glyph, name, hint]) => {
    const on = state.layers[id];
    const sym = glyph ? markerImg(glyph, c) : '<span class="ramp"></span>';
    let extra = '';
    if (on && id === 'heat') extra = `<label class="opt"><input type="checkbox" data-opt="skipSpawn"${state.skipSpawn ? ' checked' : ''}>${tr('skipSpawn')}</label>`;
    if (on && id === 'deaths') extra = chips('groups', Object.keys(GROUP_NAMES).filter(g => KILLS.some(k => k.group === g && k.vehicle == null)), g => GROUP_NAMES[g]);
    if (on && id === 'vehicles' && KINDS.length > 1) extra = chips('kinds', KINDS, k => 'kind_' + k);
    return `<div class="layer${on ? ' on' : ''}"><label title="${hint ? esc(tr(hint)) : ''}"><span class="sym">${sym}</span>` +
      `<span class="t">${tr(name)}</span><span class="n">${on ? layerCount(id) : ''}</span>` +
      `<input type="checkbox" class="switch" data-layer="${id}"${on ? ' checked' : ''}></label>${extra}</div>`;
  }).join('');
}

function renderLists() {
  const tabs = [['players', 'listPlayers'], ['vehicles', 'listVehicles'], ['kills', 'listKills']];
  let body = '';
  const cur = (type, id) => same(state.sel, { type, id }) ? ' aria-current="true"' : '';
  if (state.list === 'players') {
    body = D.teams.map(t => {
      const ps = D.players.map((p, i) => [p, i]).filter(([p]) => p.team === t.number && ofTeam(p.team)).sort((a, b) => stat(b[0], 'score') - stat(a[0], 'score'));
      if (!ps.length) return '';
      return `<div class="sub"><span style="color:${teamColor(t.number)}">${esc(teamName(t.number))}</span><span>${tr('killsDeaths')}</span></div>` + ps.map(([p, i]) => {
        const kills = KILLS.filter(k => k.byPlayer === i && k.vehicle == null && enemyKill(k) && inTime(k.t)).length;
        const deaths = KILLS.filter(k => k.player === i && k.vehicle == null && inTime(k.t)).length;
        return `<button class="row" data-type="player" data-id="${i}"${cur('player', i)}>` +
          `<span class="t${p.bot ? ' m' : ''}">${esc(p.name)}</span>${p.platoon ? `<span class="pl" title="${esc(tr('platoon', p.platoon))}">${esc(p.platoon)}</span>` : ''}` +
          `<span class="r num">${kills} / ${deaths}</span></button>`;
      }).join('');
    }).join('');
  } else if (state.list === 'vehicles') {
    const vs = D.vehicles.map((v, i) => [v, i]).filter(([v]) => v.player >= 0 && ofTeam(v.team) && overlaps(v.spawned, v.destroyed));
    body = vs.length ? vs.map(([v, i]) =>
      `<button class="row" data-type="vehicle" data-id="${i}"${cur('vehicle', i)}>${markerImg(vehicleGlyph(v), teamColor(v.team), !!v.destroyed)}` +
      `<span class="t">${esc(vehicleName(v))}<small>${esc(playerName(v.player))}</small></span>` +
      `<span class="r ${v.destroyed ? 'bad' : 'good'}">${clock(v.spawned)}<small>${v.destroyed ? tr('knockedOut') : tr('survived')}</small></span></button>`).join('')
      : `<p class="empty">${tr('noVehicles')}</p>`;
  } else {
    const LIMIT = 300;
    const ks = KILLS.map((k, i) => [k, i]).filter(([k]) => inTime(k.t) && (ofTeam(k.team) || ofTeam(k.byTeam)) && (!REL || REL.kills.has(KILLS.indexOf(k)) || state.sel.type === 'kill'));
    body = ks.length ? ks.slice(0, LIMIT).map(([k, i]) => {
      const victim = k.vehicle != null ? vehicleName(D.vehicles[k.vehicle]) : playerName(k.player);
      return `<button class="row kill" data-type="kill" data-id="${i}"${cur('kill', i)}><span class="r num m">${clock(k.t)}</span>` +
        `<span class="t">${k.by >= 0 && k.by !== k.soldier ? colored(playerName(k.byPlayer), k.byTeam) + ' → ' : ''}${colored(victim, k.team)}` +
        `<small>${esc(weaponName(k.weapon))}${k.headshot ? ` · ${tr('headshot')}` : ''}</small></span></button>`;
    }).join('') + (ks.length > LIMIT ? `<p class="empty">${tr('moreKills', ks.length - LIMIT)}</p>` : '')
      : `<p class="empty">${tr('noKills')}</p>`;
  }
  $('lists').innerHTML = `<div class="seg small" id="listSeg">${tabs.map(([id, key]) =>
    `<button data-list="${id}" aria-pressed="${state.list === id}">${tr(key)}</button>`).join('')}</div><div class="list">${body}</div>`;
  const row = state.sel && $('lists').querySelector('[aria-current="true"]');
  if (row) row.scrollIntoView({ block: 'nearest' });
}

$('panel').addEventListener('click', e => {
  const seg = e.target.closest('#teamSeg button');
  if (seg) { state.team = +seg.dataset.v; renderPanel(); renderTop(); draw(); return; }
  const list = e.target.closest('[data-list]');
  if (list) { state.list = list.dataset.list; renderLists(); return; }
  const g = e.target.closest('[data-groups], [data-kinds]');
  if (g) {
    const [set, id] = g.dataset.groups ? ['groups', g.dataset.groups] : ['kinds', g.dataset.kinds];
    state[set].has(id) ? state[set].delete(id) : state[set].add(id);
    renderLayers(); draw(); return;
  }
  const row = e.target.closest('[data-type]');
  if (row) select({ type: row.dataset.type, id: +row.dataset.id }, true);
});
$('panel').addEventListener('change', e => {
  const layer = e.target.dataset.layer, opt = e.target.dataset.opt;
  if (layer) state.layers[layer] = e.target.checked;
  else if (opt) state[opt] = e.target.checked;
  else return;
  renderLayers(); draw();
});
$('panel').addEventListener('mouseover', e => {
  const row = e.target.closest('[data-type="vehicle"]');
  const obj = row ? { type: 'vehicle', id: +row.dataset.id } : null;
  if (!same(obj, state.hot) && (obj || state.hot)) { state.hot = obj; draw(); }
});

// ---------- battle course
// The sectors of the battle as a row of stages, the kills of each team in 10-second
// buckets under it, and a time axis. A click on a sector shows it; dragging selects any
// part of the battle.
const timeline = (() => {
  const tc = $('timeline'), g = tc.getContext('2d');
  tc.title = tr(SECTORS.length ? 'stripHint' : 'stripHintNoSectors');
  const BUCKET = 10, LEFT = 14, RIGHT = 14, SECT = 30, KILLS_H = 40, AXIS = 22;
  const n = Math.max(1, Math.ceil(END / BUCKET));
  const kills = { 1: new Array(n).fill(0), 2: new Array(n).fill(0) };
  for (const k of KILLS) if (k.vehicle == null && enemyKill(k) && kills[k.byTeam]) kills[k.byTeam][Math.min(n - 1, Math.floor(k.t / BUCKET))]++;
  const peak = Math.max(1, ...kills[1], ...kills[2]);
  const top = SECTORS.length ? SECT + 8 : 0, height = 6 + top + KILLS_H + AXIS;

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

    const kt = 6 + top, mid = kt + KILLS_H / 2, amp = KILLS_H / 2 - 2, bw = Math.max(1, (w - LEFT - RIGHT) / n - 1);
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
      g.fillRect(x0, kt - 2, x1 - x0, KILLS_H + 4);
      g.fillStyle = P.text; g.fillRect(x0 - 1, kt - 2, 2, KILLS_H + 4); g.fillRect(x1 - 1, kt - 2, 2, KILLS_H + 4);
    }
    if (cursor != null) {
      const x = xOf(cursor), label = clock(cursor);
      g.font = `12px ${P.body}`;
      const lw = g.measureText(label).width + 10, lx = Math.min(Math.max(x - lw / 2, 2), w - lw - 2);
      g.fillStyle = P.text; g.globalAlpha = 0.5; g.fillRect(x, kt - 2, 1, KILLS_H + 4); g.globalAlpha = 1;
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
  if (ready) { renderLayers(); renderLists(); renderCard(); }
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
  sprites.clear();
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
