import {test} from 'node:test';
import assert from 'node:assert/strict';
import {normalizeRouteIds, normalizeRouteOrder, appendRoute, removeRoute, moveRoute} from './route-order.js';

const config = {sets:[{id:'s1'},{id:'s2'}],rules:[{id:'r1'},{id:'r2'}]};
const labels = order => order.map(ref => `${ref.kind}:${ref.id}`);

test('legacy configuration migrates sets first, then rules', () => {
  assert.deepEqual(labels(normalizeRouteOrder(config)), ['set:s1','set:s2','rule:r1','rule:r2']);
  assert.equal(config.routeOrder, undefined);
});

test('valid cross-kind order is retained; duplicates and stale entries are removed', () => {
  const order = [{kind:'rule',id:'r2'},{kind:'set',id:'s1'},{kind:'rule',id:'r2'},
    {kind:'set',id:'gone'},{kind:'bogus',id:'r1'},{kind:'rule',id:'r1'}];
  assert.deepEqual(labels(normalizeRouteOrder({...config,routeOrder:order})),
    ['rule:r2','set:s1','rule:r1','set:s2']);
});

test('new records append while edits keep position', () => {
  const existing = {...config,routeOrder:[{kind:'rule',id:'r1'},{kind:'set',id:'s1'}]};
  const before = normalizeRouteOrder(existing);
  assert.deepEqual(labels(appendRoute(existing,'rule','r1')),labels(before));
  assert.deepEqual(labels(appendRoute(existing,'set','s2')),labels(before));
  const created = {...existing,rules:[...config.rules,{id:'r3'}],routeOrder:before};
  assert.equal(labels(normalizeRouteOrder(created)).at(-1),'rule:r3');
});

test('remove and move work across kinds without mutating source', () => {
  const ordered = {...config,routeOrder:normalizeRouteOrder(config)};
  assert.deepEqual(labels(moveRoute(ordered,'rule','r1',-1)),['set:s1','rule:r1','set:s2','rule:r2']);
  assert.deepEqual(labels(moveRoute(ordered,'set','s1',-1)),labels(ordered.routeOrder));
  const deleted = {...ordered,sets:[{id:'s2'}]};
  assert.deepEqual(labels(removeRoute(deleted,'set','s1')),['set:s2','rule:r1','rule:r2']);
  assert.deepEqual(labels(ordered.routeOrder),['set:s1','set:s2','rule:r1','rule:r2']);
});

test('legacy rules with missing and duplicate IDs retain every row deterministically', () => {
  const legacy={sets:[],rules:[{id:'legacy-rule-1',value:'reserved'},
    {value:'missing'},{id:'same',value:'first'},{id:'same',value:'second'}]};
  const migrated=normalizeRouteIds(legacy);
  assert.deepEqual(migrated.rules.map(rule=>rule.id),
    ['legacy-rule-1','legacy-rule-2','same','legacy-rule-3']);
  assert.deepEqual(labels(normalizeRouteOrder(migrated)),
    ['rule:legacy-rule-1','rule:legacy-rule-2','rule:same','rule:legacy-rule-3']);
  assert.equal(legacy.rules[1].id,undefined);
});
