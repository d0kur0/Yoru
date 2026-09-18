import {test} from 'node:test';
import assert from 'node:assert/strict';
import {newRouteSets} from './route-sets-file.js';
const set={id:'local',name:'Work',enabled:true,domains:['+.company.example'],processes:[],cidrs:[],resolvers:['10.20.0.53'],action:'DIRECT',realIP:true,bypassTUN:true};
test('repeat imports skip identical sets regardless of id or JSON property order',()=>{
 assert.deepEqual(newRouteSets([set],[{...set,id:'imported'}]),[]);
 assert.equal(newRouteSets([],[set,{...set,id:'other'}]).length,1);
});
test('changed sets append independently without replacing or mutating existing settings',()=>{
 const existing=structuredClone(set),incoming={...set,domains:['+.other.example']};
 const additions=newRouteSets([existing],[incoming]);
 assert.equal(additions.length,1);assert.notEqual(additions[0].id,existing.id);
 assert.deepEqual(existing,set);assert.equal(incoming.id,'local');
 additions[0].domains.push('extra.example');assert.equal(incoming.domains.length,1);
});
test('disabled and enabled copies remain distinct, source order is retained',()=>{
 const off={...set,id:'off',enabled:false};
 assert.deepEqual(newRouteSets([], [off,set]).map(s=>s.enabled),[false,true]);
});

test('keyword-only differences survive import and legacy files still deduplicate',()=>{
 assert.deepEqual(newRouteSets([set],[{...set,keywords:[]}]),[]);
 const imported=newRouteSets([set],[{...set,keywords:['ubisoft']}]);
 assert.equal(imported.length,1);assert.deepEqual(imported[0].keywords,['ubisoft']);
});
