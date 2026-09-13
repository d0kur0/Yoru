import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServerLatency} from './server-latency.js';
test('all nodes are probed, cached and refreshed after interval',async()=>{
 let time=1000;const calls=[];let saved;const storage={getItem:()=>saved,setItem:(_,v)=>saved=v};
 const monitor=createServerLatency(async id=>{calls.push(id);return 42},storage,()=>time);
 const servers=[{id:'a'},{id:'b'}];
 await monitor.tick(true,'session',servers,'url');await monitor.tick(true,'session',servers,'url');await monitor.tick(true,'session',servers,'url');
 assert.deepEqual(calls,['a','b']);assert.equal(monitor.get('a','url').value,42);
 assert.equal(createServerLatency(()=>{},storage).get('b','url').value,42);
 assert.equal(monitor.get('a','changed'),null);
 time+=60000;await monitor.tick(true,'session',servers,'url');assert.deepEqual(calls,['a','b','a']);
});
test('late reply after disconnect is discarded',async()=>{
 let resolve;const monitor=createServerLatency(()=>new Promise(r=>resolve=r),null);const nodes=[{id:'a'}];
 const pending=monitor.tick(true,'s',nodes,'url');await monitor.tick(false,'s',nodes,'url');resolve(20);await pending;assert.equal(monitor.get('a','url'),null);
});
