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

test('legacy reconnect sentinel is discarded and replaced with a fresh measurement',async()=>{
 const storage={getItem:()=>JSON.stringify({a:{value:-2,time:1,url:'url'}}),setItem:()=>{}};
 const monitor=createServerLatency(async()=>37,storage);
 assert.equal(monitor.get('a','url'),null);
 await monitor.tick(true,'session',[{id:'a'}],'url');
 assert.equal(monitor.get('a','url').value,37);
});

const flush=()=>new Promise(resolve=>setImmediate(resolve));
test('manual refresh marks every row immediately, drains queue and publishes each result',async()=>{
 const calls=[],resolvers=[],states=[];
 const storage={getItem:()=>JSON.stringify({a:{value:90,time:1,url:'url'},b:{value:80,time:1,url:'url'}}),setItem:()=>{}};
 const monitor=createServerLatency(id=>{calls.push(id);return new Promise(resolve=>resolvers.push(resolve));},storage);
 const nodes=[{id:'a'},{id:'b'}];
 const refresh=monitor.refresh(true,'s',nodes,'url',()=>states.push([monitor.activity('a'),monitor.activity('b')]));
 assert.equal(monitor.refreshing,true);
 assert.equal(monitor.activity('a'),'measuring');assert.equal(monitor.activity('b'),'queued');
 assert.deepEqual(calls,['a']);
 await monitor.tick(true,'s',nodes,'url');await monitor.refresh(true,'s',nodes,'url');
 assert.deepEqual(calls,['a'],'polling and repeated clicks must not duplicate probes');
 resolvers.shift()(31);await flush();
 assert.equal(monitor.get('a','url').value,31);assert.equal(monitor.activity('a'),null);
 assert.equal(monitor.activity('b'),'measuring');assert.deepEqual(calls,['a','b']);
 resolvers.shift()(44);await refresh;
 assert.equal(monitor.get('b','url').value,44);assert.equal(monitor.refreshing,false);
 assert.equal(monitor.activity('b'),null);assert.deepEqual(states[1],['queued','queued']);
});
test('manual refresh reuses current automatic probe and then checks remaining nodes',async()=>{
 const calls=[],resolvers=[];
 const monitor=createServerLatency(id=>{calls.push(id);return new Promise(resolve=>resolvers.push(resolve));},null);
 const nodes=[{id:'a'},{id:'b'}];
 const automatic=monitor.tick(true,'s',nodes,'url');
 const manual=monitor.refresh(true,'s',nodes,'url');
 resolvers.shift()(20);await automatic;await flush();
 assert.deepEqual(calls,['a','b']);assert.equal(monitor.activity('a'),null);
 resolvers.shift()(30);await manual;assert.equal(monitor.refreshing,false);
});
test('disconnect cancels manual queue and ignores late result',async()=>{
 let resolve;const calls=[];
 const monitor=createServerLatency(id=>{calls.push(id);return new Promise(r=>resolve=r);},null);
 const nodes=[{id:'a'},{id:'b'}];
 const manual=monitor.refresh(true,'s',nodes,'url');
 await monitor.tick(false,'s',nodes,'url');
 assert.equal(monitor.refreshing,false);assert.equal(monitor.activity('a'),null);assert.equal(monitor.activity('b'),null);
 resolve(20);await manual;assert.equal(monitor.get('a','url'),null);assert.deepEqual(calls,['a']);
});
test('failed probe clears its spinner and does not stop remaining queue',async()=>{
 const calls=[];const monitor=createServerLatency(async id=>{calls.push(id);if(id==='a')throw Error('timeout');return 22;},null);
 await monitor.refresh(true,'s',[{id:'a'},{id:'b'}],'url');
 assert.deepEqual(calls,['a','b']);assert.equal(monitor.get('a','url').value,-1);assert.equal(monitor.get('b','url').value,22);
 assert.equal(monitor.refreshing,false);assert.equal(monitor.activity('a'),null);
});
test('changing URL during manual refresh discards old results and queue',async()=>{
 let resolve;const calls=[];const monitor=createServerLatency(id=>{calls.push(id);return new Promise(r=>resolve=r);},null);
 const nodes=[{id:'a'},{id:'b'}];const manual=monitor.refresh(true,'s',nodes,'old');
 await monitor.tick(true,'s',nodes,'new');resolve(80);await manual;
 assert.equal(monitor.get('a','old'),null);assert.equal(monitor.refreshing,false);assert.deepEqual(calls,['a']);
});
