import {test} from 'node:test';
import assert from 'node:assert/strict';
import {serverFromForm} from './server-editor.js';
const original={id:'sub-node',subscription:'source',name:'Original',host:'server.example',port:443,secret:'old',protocol:'Hysteria2',transport:'Imported',options:{obfs:'salamander','obfs-password':'obfs-secret',sni:'old.example'}};
const fields=()=>new Map([['name','Edited'],['host','new.example'],['port','8443'],['secret','new-secret'],['sni','']]);
test('editing preserves imported options and does not mutate the source',()=>{
 const result=serverFromForm(original,original,fields());
 assert.equal(result.id,original.id);assert.equal(result.subscription,'source');assert.equal(result.secret,'new-secret');assert.equal(result.options['obfs-password'],'obfs-secret');assert.equal(result.options.sni,undefined);assert.equal(original.options.sni,'old.example');assert.equal(original.secret,'old');
});
test('independent copy gets a new identity and keeps protocol settings',()=>{
 const f=fields();f.set('independent','on');const result=serverFromForm(original,original,f);
 assert.notEqual(result.id,original.id);assert.equal(result.subscription,undefined);assert.equal(result.options.obfs,'salamander');assert.equal(original.subscription,'source');
});
test('invalid port or blank secret cannot be saved',()=>{
 const f=fields();f.set('port','65536');assert.throws(()=>serverFromForm(original,original,f));f.set('port','443');f.set('secret',' ');assert.throws(()=>serverFromForm(original,original,f));
});
