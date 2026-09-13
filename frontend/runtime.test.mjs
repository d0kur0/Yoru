import {test} from 'node:test';
import assert from 'node:assert/strict';
import {emptyConfig,bytes,sample,decodeBackup} from './src/reference/runtime.js';

test('cold start contains no fictional servers or enabled autoconnect',()=>{
 const c=emptyConfig();assert.deepEqual(c.servers,[]);assert.deepEqual(c.subscriptions,[]);assert.equal(c.settings.autoConnect,false);assert.equal(c.settings.autostart,false);
 assert.equal(c.settings.mode,'rule');assert.equal(c.defaultAction,'DIRECT');
});
test('traffic is calculated from counters and resets between sessions',()=>{
 const first=sample(null,{running:true,started:1,downloadTotal:100,uploadTotal:10},1000);
 const second=sample(first,{running:true,started:1,downloadTotal:2148,uploadTotal:522},3000);
 assert.equal(second.down,1024);assert.equal(second.up,256);
 assert.equal(sample(second,{running:true,started:2,downloadTotal:0,uploadTotal:0},4000).down,0);
 assert.equal(sample(second,{running:false},4000).up,0);
});
test('backup preserves credentials and produces an independent copy',()=>{
 const c=emptyConfig();c.servers.push({secret:'test-key'});const restored=decodeBackup({format:'mihomo-desktop',version:1,config:c});
 assert.deepEqual(restored,c);restored.servers[0].secret='changed';assert.equal(c.servers[0].secret,'test-key');
 assert.throws(()=>decodeBackup({format:'unknown',version:1,config:c}));
});
test('byte formatting uses measured values',()=>{assert.equal(bytes(0),'0 Б');assert.equal(bytes(1024),'1.0 КБ');});
