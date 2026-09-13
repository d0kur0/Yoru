import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createLatencyMonitor} from './src/reference/latency-monitor.js';

test('connect probes immediately, refreshes every minute and stops offline', async () => {
 let time=100, calls=0;
 const m=createLatencyMonitor(async()=>{calls++;return 42;},()=>time);
 await m.tick(false,0,'a'); assert.equal(calls,0);
 await m.tick(true,1,'a'); assert.equal(m.value,42);
 await m.tick(true,1,'a'); assert.equal(calls,1);
 time+=60000; await m.tick(true,1,'a'); assert.equal(calls,2);
 await m.tick(false,0,'a'); assert.equal(m.value,null);
 await m.tick(true,2,'a'); assert.equal(calls,3);
 await m.tick(true,2,'b'); assert.equal(calls,4);
});

test('deduplicates pending probes and ignores old session results', async()=>{
 const replies=[];
 const m=createLatencyMonitor(()=>new Promise(resolve=>replies.push(resolve)));
 const old=m.tick(true,1,'a'); await Promise.resolve();
 m.tick(true,1,'a'); assert.equal(replies.length,1);
 const next=m.tick(true,2,'b'); await Promise.resolve();
 replies[1](25); await next; replies[0](99); await old;
 assert.equal(m.value,25);
});

test('failed probes show timeout and retry on the next interval',async()=>{
 let time=1;
 const m=createLatencyMonitor(async()=>{throw Error('offline');},()=>time);
 await m.tick(true,1,'a'); assert.equal(m.value,-1);
 time+=60000; await m.tick(true,1,'a'); assert.equal(m.value,-1);
});
