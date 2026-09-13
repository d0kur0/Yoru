// Results belong to a core session and node; late replies cannot cross reconnects.
export function createLatencyMonitor(probe, now = Date.now) {
 let key = '', value = null, pending = false, due = 0, generation = 0;
 return {
  get value() { return value; },
  tick(running, started, id) {
   const next = running && id ? `${started}:${id}` : '';
   if (next !== key) {
    key = next; value = null; due = 0; pending = false; generation++;
   }
   if (!key || pending || now() < due) return;
   pending = true;
   const request = generation;
   return Promise.resolve().then(() => probe(id)).then(result => {
    if (request === generation) value = result;
   }, () => {
    if (request === generation) value = -1;
   }).finally(() => {
    if (request === generation) { pending = false; due = now() + (value < 0 ? 10000 : 60000); }
   });
  }
 };
}
