// The core already health-checks VPN groups; the list reuses that history and
// probes only nodes without a recent sample, in one concurrent batch.
export const HISTORY_INTERVAL = 10000;
export const STALE_AFTER = 5 * 60000;

export function createServerLatency({ measure, history }, storage, now = Date.now) {
 let results = {};
 try { results = JSON.parse(storage?.getItem('server-latency-v1') || '{}'); } catch {}
 let session = '', generation = 0, batch = null, historyDue = 0, historyPending = false, lastServers = [];
 const tried = new Map();
 function persist(servers) {
  const ids = new Set(servers.map(s => s.id));
  results = Object.fromEntries(Object.entries(results).filter(([id]) => ids.has(id)));
  try { storage?.setItem('server-latency-v1', JSON.stringify(results)); } catch {}
 }
 function sync(running, key, servers, url, onChange) {
  lastServers = servers;
  // Node definitions stay in memory only; changed nodes invalidate in-flight replies.
  const next = running ? JSON.stringify([key, url, servers.map(({ latency, ...s }) => s)]) : '';
  if (next === session) return;
  session = next;
  generation++;
  batch = null;
  historyDue = 0;
  historyPending = false;
  tried.clear();
  onChange();
 }
 async function readHistory(url, onChange) {
  if (!session || historyPending || now() < historyDue) return;
  historyPending = true;
  const token = generation;
  try {
   const samples = await history();
   if (token !== generation) return;
   let changed = false;
   for (const [id, sample] of Object.entries(samples || {})) {
    const old = results[id];
    if (old?.url === url && old.time >= sample.time) continue;
    results[id] = { value: sample.delay, time: sample.time, url };
    changed = true;
   }
   if (changed) { persist(lastServers); onChange(); }
  } catch {} finally {
   if (token === generation) { historyPending = false; historyDue = now() + HISTORY_INTERVAL; }
  }
 }
 function stale(id, url) {
  const r = results[id];
  const fresh = r?.url === url && r.value !== -2 && now() - r.time < STALE_AFTER;
  return !fresh && (!tried.has(id) || now() - tried.get(id) >= STALE_AFTER);
 }
 function run(servers, url, onChange, ids) {
  if (!session || batch || !ids.length) return batch?.work;
  const token = generation;
  const current = { ids: new Set(ids), work: null };
  batch = current;
  for (const id of ids) tried.set(id, now());
  onChange();
  current.work = Promise.resolve()
   .then(() => measure(ids))
   .then(delays => {
    if (token !== generation) return;
    const time = now();
    for (const id of ids) results[id] = { value: Number(delays?.[id] ?? -1), time, url };
    persist(servers);
   }, () => {
    if (token !== generation) return;
    const time = now();
    for (const id of ids) results[id] = { value: -1, time, url };
   })
   .finally(() => {
    if (batch === current) { batch = null; onChange(); }
   });
  return current.work;
 }
 return {
  get(id, url) {
   const r = results[id];
   return r?.url === url && r.value !== -2 ? r : null;
  },
  activity(id) { return batch?.ids.has(id) ? 'measuring' : null; },
  get refreshing() { return batch !== null; },
  // Background step: cheap history read; probes only stale nodes while the list is visible.
  async tick(running, key, servers, url, onChange = () => {}, { probe = false } = {}) {
   sync(running, key, servers, url, onChange);
   await readHistory(url, onChange);
   if (probe) return run(servers, url, onChange, servers.map(s => s.id).filter(id => stale(id, url)));
  },
  async refresh(running, key, servers, url, onChange = () => {}) {
   sync(running, key, servers, url, onChange);
   return run(servers, url, onChange, servers.map(s => s.id));
  },
 };
}
