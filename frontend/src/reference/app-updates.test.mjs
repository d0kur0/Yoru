import test from 'node:test';
import assert from 'node:assert/strict';
import {updatePresentation} from './app-updates.js';

test('updater offers installation only after verification',()=>{
 for(const phase of ['idle','checking','available','downloading','current','error']){
  assert.notEqual(updatePresentation({phase,version:'1.3.0'},'1.2.2').action,'install');
 }
 assert.equal(updatePresentation({phase:'ready',version:'1.3.0'},'1.2.2').action,'install');
});
test('download progress is bounded and controls disabled',()=>{
 const view=updatePresentation({phase:'downloading',downloaded:120,total:100},'1.2.2');
 assert.equal(view.text,'Загрузка 100%');
 assert.equal(view.disabled,true);
});
test('macOS describes installer handoff instead of automatic replacement',()=>{
 assert.equal(updatePresentation({phase:'ready',platform:'darwin',version:'1.3.0'},'1.2.2').label,'Открыть установщик');
});
test('failure provides an explicit retry and visible reason',()=>{
 const view=updatePresentation({phase:'error',error:'Нет сети'},'1.2.2');
 assert.equal(view.text,'Нет сети');
 assert.equal(view.action,'check');
});
