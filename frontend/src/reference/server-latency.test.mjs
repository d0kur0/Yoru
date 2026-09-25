import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServerLatency} from './server-latency.js';
const flush=()=>new Promise(r=>setImmediate(r));
const nodes=n=>Array.from({length:n},(_,i)=>({id:String(i)}));
test('automatic batch checks all nodes without starving the tail and caches results',async()=>{
 let time=1000,saved;const calls=[];
 const monitor=createServerLatency(async id=>{calls.push(id);time+=31000;return -1;},{getItem:()=>saved,setItem:(_,v)=>saved=v},()=>time);
 await monitor.tick(true,'s',nodes(20),'url');
 assert.equal(calls.length,20);assert.equal(new Set(calls).size,20);
 assert.equal(createServerLatency(()=>{},{getItem:()=>saved}).get('19','url').value,-1);
 assert.equal(monitor.get('0','other'),null);
});
test('pool limits concurrency to four, publishes each result and distinguishes queued rows',async()=>{
 const calls=[],resolve=new Map();let active=0,max=0;
 const monitor=createServerLatency(id=>{calls.push(id);max=Math.max(max,++active);return new Promise(r=>resolve.set(id,v=>{active--;r(v);}));});
 const work=monitor.refresh(true,'s',nodes(6),'url');await flush();
 assert.equal(calls.length,4);assert.equal(monitor.activity('4'),'queued');assert.equal(monitor.activity('0'),'measuring');
 await monitor.tick(true,'s',nodes(6),'url');await monitor.refresh(true,'s',nodes(6),'url');
 assert.equal(calls.length,4);
 resolve.get('0')(25);await flush();assert.equal(monitor.get('0','url').value,25);assert.equal(monitor.progress.completed,1);assert.equal(monitor.activity('4'),'measuring');
 for(const id of ['1','2','3','4'])resolve.get(id)(30);await flush();resolve.get('5')(40);await work;
 assert.equal(max,4);assert.equal(monitor.refreshing,false);assert.equal(calls.length,6);
});
test('disconnect cancels queued work and discards late replies',async()=>{
 const resolvers=[];const monitor=createServerLatency(()=>new Promise(r=>resolvers.push(r)));
 const work=monitor.tick(true,'s',nodes(7),'url');await flush();await monitor.tick(false,'s',nodes(7),'url');
 assert.equal(monitor.activity('0'),null);assert.equal(monitor.refreshing,false);
 resolvers.forEach(r=>r(20));await work;assert.equal(monitor.get('0','url'),null);assert.equal(resolvers.length,4);
});
test('new URL waits for old probes without exceeding pool and ignores old replies',async()=>{
 const resolvers=[];let active=0,max=0;
 const monitor=createServerLatency(()=>{max=Math.max(max,++active);return new Promise(r=>resolvers.push(v=>{active--;r(v);}));});
 const old=monitor.tick(true,'s',nodes(4),'old');await flush();const fresh=monitor.tick(true,'s',nodes(4),'new');await flush();
 assert.equal(resolvers.length,4);resolvers.splice(0).forEach(r=>r(50));await old;await flush();
 assert.equal(monitor.get('0','old'),null);resolvers.splice(0).forEach(r=>r(10));await fresh;
 assert.equal(max,4);assert.equal(monitor.get('0','new').value,10);
});
test('failed probe does not stop batch and success respects refresh interval',async()=>{
 let time=1000,calls=0;const monitor=createServerLatency(async id=>{calls++;if(id==='0')throw Error('timeout');return 22;},null,()=>time);
 await monitor.tick(true,'s',nodes(2),'url');assert.equal(monitor.get('0','url').value,-1);assert.equal(monitor.get('1','url').value,22);
 await monitor.tick(true,'s',nodes(2),'url');assert.equal(calls,2);
 time+=30000;await monitor.tick(true,'s',nodes(2),'url');assert.equal(calls,3);
});
test('legacy reconnect sentinel is not shown',async()=>{
 const monitor=createServerLatency(async()=>37,{getItem:()=>JSON.stringify({'0':{value:-2,time:1,url:'url'}})});
 assert.equal(monitor.get('0','url'),null);await monitor.tick(true,'s',nodes(1),'url');assert.equal(monitor.get('0','url').value,37);
});
