import {test} from 'node:test';
import assert from 'node:assert/strict';
import {filterLogs} from './src/reference/log-filter.js';

test('log filters include the selected severity and every higher severity',()=>{
 const levels=['debug','info','warning','error','fatal','panic'];
 const lines=levels.map(level=>`level=${level} msg="message"`);
 for(const [index,level] of levels.slice(0,4).entries()){
  assert.equal(filterLogs(lines.join('\n'),'',level),lines.slice(index).join('\n'));
 }
});
test('warning aliases and quoted levels work; message text is not a severity',()=>{
 const lines=['level="warn" msg="network"',"level='error' msg=\"network\"",'level=debug msg="error network"'];
 assert.equal(filterLogs(lines.join('\n'),'NETWORK','warning'),lines.slice(0,2).join('\n'));
 assert.equal(filterLogs('unstructured message','',''),'unstructured message');
});
