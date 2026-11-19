import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createServerLatency, HISTORY_INTERVAL, STALE_AFTER} from './server-latency.js';
const flush=()=>new Promise(r=>setImmediate(r));
const nodes=n=>Array.from({length:n},(_,i)=>({id:String(i)}));

test('core history fills the list without probing',async()=>{
 let time=1000,historyCalls=0;const measured=[];
 const monitor=createServerLatency({measure:async ids=>{measured.push(ids);return {};},history:async()=>{historyCalls++;return {'0':{delay:30,time:900},'1':{delay:-1,time:900}};}},null,()=>time);
 await monitor.tick(true,'s',nodes(2),'url',undefined,{probe:true});
 assert.equal(monitor.get('0','url').value,30);assert.equal(monitor.get('1','url').value,-1);
 assert.equal(measured.length,0);assert.equal(historyCalls,1);
 await monitor.tick(true,'s',nodes(2),'url');assert.equal(historyCalls,1);
 time+=HISTORY_INTERVAL;await monitor.tick(true,'s',nodes(2),'url');assert.equal(historyCalls,2);
});

test('nodes without recent samples are measured in one batch only while visible',async()=>{
 let time=1000;const measured=[];
 const monitor=createServerLatency({measure:async ids=>{measured.push(ids);return Object.fromEntries(ids.map(id=>[id,50]));},history:async()=>({'0':{delay:30,time:1000}})},null,()=>time);
 await monitor.tick(true,'s',nodes(3),'url');assert.equal(measured.length,0);
 await monitor.tick(true,'s',nodes(3),'url',undefined,{probe:true});
 assert.deepEqual(measured,[['1','2']]);assert.equal(monitor.get('2','url').value,50);
 time+=60000;await monitor.tick(true,'s',nodes(3),'url',undefined,{probe:true});assert.equal(measured.length,1);
 time+=STALE_AFTER;await monitor.tick(true,'s',nodes(3),'url',undefined,{probe:true});assert.equal(measured.length,2);
});

test('manual refresh measures every node at once and reports activity',async()=>{
 let release;const measured=[];
 const monitor=createServerLatency({measure:ids=>{measured.push(ids);return new Promise(r=>release=()=>r({'0':12,'1':-1}));},history:async()=>({})});
 const work=monitor.refresh(true,'s',nodes(3),'url');await flush();
 assert.deepEqual(measured,[['0','1','2']]);assert.equal(monitor.refreshing,true);assert.equal(monitor.activity('2'),'measuring');
 const again=monitor.refresh(true,'s',nodes(3),'url');await flush();assert.equal(measured.length,1);
 release();await work;await again;
 assert.equal(monitor.refreshing,false);assert.equal(monitor.get('0','url').value,12);assert.equal(monitor.get('2','url').value,-1);
});

test('disconnect discards late replies',async()=>{
 let release,saved;
 const monitor=createServerLatency({measure:()=>new Promise(r=>release=r),history:async()=>({})},{getItem:()=>saved,setItem:(_,v)=>saved=v});
 const work=monitor.refresh(true,'s',nodes(1),'url');await flush();
 await monitor.tick(false,'s',nodes(1),'url');assert.equal(monitor.refreshing,false);
 release({'0':20});await work;assert.equal(monitor.get('0','url'),null);
});

test('failed batch marks nodes as unanswered and older history does not overwrite newer probes',async()=>{
 let time=5000,fail=true,saved;
 const monitor=createServerLatency({measure:async ids=>{if(fail)throw Error('offline');return {'0':44};},history:async()=>({'0':{delay:99,time:100}})},{getItem:()=>saved,setItem:(_,v)=>saved=v},()=>time);
 await monitor.refresh(true,'s',nodes(1),'url');assert.equal(monitor.get('0','url').value,-1);
 fail=false;await monitor.refresh(true,'s',nodes(1),'url');assert.equal(monitor.get('0','url').value,44);
 time+=HISTORY_INTERVAL;await monitor.tick(true,'s',nodes(1),'url');assert.equal(monitor.get('0','url').value,44);
 assert.equal(createServerLatency({measure:async()=>({}),history:async()=>({})},{getItem:()=>saved}).get('0','url').value,44);
});

test('legacy reconnect sentinel is not shown',async()=>{
 const monitor=createServerLatency({measure:async()=>({'0':37}),history:async()=>({})},{getItem:()=>JSON.stringify({'0':{value:-2,time:1,url:'url'}})});
 assert.equal(monitor.get('0','url'),null);await monitor.refresh(true,'s',nodes(1),'url');assert.equal(monitor.get('0','url').value,37);
});
