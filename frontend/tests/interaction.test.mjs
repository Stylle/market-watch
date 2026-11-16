import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {validate,searchLocal} from '../src/preferences.js';

test('hover feedback is pointer-only and distinct from persistent selected state',()=>{
 const css=readFileSync(new URL('../src/style.css',import.meta.url),'utf8');
 assert.match(css,/@media \(hover: hover\) and \(pointer: fine\)\s*\{\s*button:hover:not\(:disabled\):not\(\[aria-pressed="true"\]\)\s*\{\s*background: var\(--subtle\)/);
 assert.match(css,/\.watch-row.selected\s*\{\s*background: var\(--selected\)/);
 const app=readFileSync(new URL('../src/App.vue',import.meta.url),'utf8');
 assert.match(app,/:aria-pressed="filter === id"/);
 assert.match(app,/:aria-pressed="period === id"/);
});

test('existing CN watchlist IDs remain importable and Chinese searchable',()=>{
 const items=[{id:'y:000300.SS',symbol:'000300',name:'沪深300',kind:'index',market:'SSE',currency:'CNY'},
 {id:'y:000001.SZ',symbol:'000001',name:'平安银行',kind:'stock',market:'SZSE',currency:'CNY'}];
 const prefs=validate({version:1,items,selected:items[0].id},[],true);
 assert.equal(prefs.selected,'y:000300.SS');assert.equal(prefs.items.length,2);
 assert.equal(searchLocal(prefs.items,'沪深300')[0].id,'y:000300.SS');
});

test('unavailable first quotes do not appear as a real zero price',()=>{
 const app=readFileSync(new URL('../src/App.vue',import.meta.url),'utf8');
 assert.match(app,/number\(\(q\?\.price \?\? 0\) > 0 \? q\?\.price : null\)/);
 assert.match(app,/number\(\(quotes\[i.id\]\?\.price \?\? 0\) > 0/);
});
