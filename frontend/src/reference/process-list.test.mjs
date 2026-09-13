import {test} from 'node:test';
import assert from 'node:assert/strict';
import {uniqueProcesses} from './process-list.js';
test('many instances collapse, distinct executable paths remain selectable',()=>{
 const items=Array.from({length:50},(_,i)=>({pid:i,name:'chrome.exe',path:'C:/Chrome/chrome.exe'}));
 items.push({pid:51,name:'chrome.exe',path:'C:/Beta/chrome.exe'},{pid:52,name:'chrome.exe',path:''});
 const result=uniqueProcesses(items,true);assert.equal(result.length,2);assert.deepEqual(new Set(result.map(p=>p.path)),new Set(['C:/Chrome/chrome.exe','C:/Beta/chrome.exe']));
});
test('Windows ignores path case, other platforms preserve it',()=>{
 const items=[{pid:1,name:'app',path:'/Apps/A'},{pid:2,name:'app',path:'/apps/a'}];assert.equal(uniqueProcesses(items,true).length,1);assert.equal(uniqueProcesses(items,false).length,2);
});
test('pathless duplicates collapse by executable name',()=>{
 assert.equal(uniqueProcesses([{pid:1,name:'protected'},{pid:2,name:'protected'}]).length,1);
});
