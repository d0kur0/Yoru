import test from 'node:test';
import assert from 'node:assert/strict';
import {createConfigSync} from './config-sync.js';

test('waits for queued saves and applies only the backend committed snapshot',async()=>{
 let unblock,revision=1,applied,calls=0;
 const queued=new Promise(resolve=>{unblock=()=>{revision=2;resolve();};});
 const sync=createConfigSync({api:{async SyncConfigURL(source,rev){calls++;assert.equal(source,'https://config.example/token');assert.equal(rev,2);return JSON.stringify({revision:3,servers:[]});}},beforeSync:()=>queued,getRevision:()=>revision,apply:next=>{applied=next;}});
 const task=sync(' https://config.example/token ');
 assert.equal(calls,0);
 assert.equal(await sync('https://config.example/other'),false);
 unblock();
 assert.equal(await task,false);
 assert.deepEqual(applied,{revision:3,servers:[]});
 assert.equal(calls,1);
});

test('failed request keeps state and allows retry with the same URL',async()=>{
 let attempts=0,applied=0;
 const sync=createConfigSync({api:{async SyncConfigURL(){if(++attempts===1)throw Error('Нет сети');return '{"revision":2}';}},beforeSync:async()=>{},getRevision:()=>1,apply:()=>applied++});
 await assert.rejects(sync('https://config.example/token'),/Нет сети/);
 assert.equal(applied,0);
 await sync('https://config.example/token');
 assert.equal(applied,1);
});

test('invalid URLs never reach the backend or expose their contents',async()=>{
 const sync=createConfigSync({api:{SyncConfigURL(){assert.fail('invalid URL fetched');}},beforeSync:async()=>{},getRevision:()=>1,apply:()=>assert.fail()});
 for(const source of ['http://example.com/private-token','https://user:password@example.com/token','https://example.com/#private-token','invalid-private-token']){
  await assert.rejects(sync(source),error=>!error.message.includes('private-token'));
 }
});
