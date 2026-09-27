// Route order is independent of the records so editing a record never moves it.
export function normalizeRouteIds(config) {
  const reserved = new Set((config.rules || []).map(rule => rule.id).filter(id => typeof id === 'string' && id));
  const seen = new Set();
  let serial = 1;
  return {
    ...config,
    rules: (config.rules || []).map(rule => {
      const id = typeof rule.id === 'string' ? rule.id : '';
      if (id && !seen.has(id)) { seen.add(id); return rule; }
      let replacement;
      do { replacement = `legacy-rule-${serial++}`; } while (reserved.has(replacement));
      reserved.add(replacement);
      return {...rule, id: replacement};
    }),
  };
}
export function normalizeRouteOrder(config) {
  const sets = new Set((config.sets || []).map(item => item.id).filter(Boolean));
  const rules = new Set((config.rules || []).map(item => item.id).filter(Boolean));
  const seen = new Set();
  const result = [];
  const add = (kind, id) => {
    const key = `${kind}:${id}`;
    if (typeof id !== 'string' || !id || seen.has(key) || !(kind === 'set' ? sets : rules).has(id)) return;
    seen.add(key);
    result.push({kind, id});
  };
  for (const ref of Array.isArray(config.routeOrder) ? config.routeOrder : []) {
    if (ref?.kind === 'set' || ref?.kind === 'rule') add(ref.kind, ref.id);
  }
  for (const set of config.sets || []) add('set', set.id);
  for (const rule of config.rules || []) add('rule', rule.id);
  return result;
}

export function appendRoute(config, kind, id) {
  const order = normalizeRouteOrder(config);
  if (!order.some(ref => ref.kind === kind && ref.id === id)) order.push({kind, id});
  return order;
}

export function removeRoute(config, kind, id) {
  return normalizeRouteOrder(config).filter(ref => ref.kind !== kind || ref.id !== id);
}

export function moveRoute(config, kind, id, direction) {
  const order = normalizeRouteOrder(config);
  const index = order.findIndex(ref => ref.kind === kind && ref.id === id);
  const destination = index + direction;
  if (index < 0 || destination < 0 || destination >= order.length) return order;
  [order[index], order[destination]] = [order[destination], order[index]];
  return order;
}
