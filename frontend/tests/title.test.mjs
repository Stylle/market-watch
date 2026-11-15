import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

test('document title remains intact Unicode after HTML entity decoding',()=>{
 const html=readFileSync(new URL('../index.html',import.meta.url),'utf8');
 assert.equal(html.includes('\uFFFD'),false);
 assert.match(html,/<meta charset="UTF-8"/i);
 const title=html.match(/<title>(.*?)<\/title>/s)?.[1].replace(/&#(\d+);/g,(_,n)=>String.fromCodePoint(Number(n)));
 assert.equal(title,'\u89c2\u5e02 \u00b7 Market Watch');
});
