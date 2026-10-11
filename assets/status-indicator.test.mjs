import test from 'node:test';
import assert from 'node:assert/strict';
import {createRequire} from 'node:module';
const tracker = createRequire(import.meta.url)('./status-indicator.js');

test('page failure survives unrelated successes; matching retry clears it', () => {
 let visible;
 const t = tracker('page', 1, 'hb-1', id => visible=id);
 assert.equal(visible,'hb-1');
 t.finish(t.begin('fragment'),0,'hb-2');
 assert.equal(visible,'hb-1');
 t.finish(t.begin('page'),0,'hb-3');
 assert.equal(visible,'');
});
test('multiple fragment failures require each failing request to recover', () => {
 let visible;
 const t = tracker('page',0,'hb-1',id=>visible=id);
 t.finish(t.begin('a'),1,'hb-2');
 t.finish(t.begin('b'),2,'hb-3');
 t.finish(t.begin('b'),0,'hb-4');
 assert.equal(visible,'hb-2');
 t.finish(t.begin('a'),0,'hb-5');
 assert.equal(visible,'');
});
test('late responses cannot overwrite the latest retry; invalid headers are ignored', () => {
 let visible;
 const t = tracker('page',0,'hb-1',id=>visible=id);
 const old=t.begin('a'), current=t.begin('a');
 t.finish(current,1,'hb-3');
 t.finish(old,0,'hb-2');
 assert.equal(visible,'hb-3');
 t.finish(t.begin('a'),null,null);
 assert.equal(visible,'hb-3');
 t.finish(t.begin('a'),0,'javascript:bad');
 assert.equal(visible,'hb-3');
 t.finish(t.begin('a'),0,'hb-4');
 assert.equal(visible,'');
});

import vm from 'node:vm';
import {readFileSync} from 'node:fs';
function browserHarness(options={}) {
 const elements=[];
 function element(tag) {
  const attrs={}, listeners={};
  const e={tag,style:{},dataset:{},append(){},attachShadow(){return element('shadow')},
   setAttribute(k,v){attrs[k]=v},getAttribute(k){return attrs[k]},addEventListener(k,v){listeners[k]=v},listeners};
  elements.push(e);return e;
 }
 class XHR extends EventTarget {
  constructor(){super();this.headers={};this.calls=[];this.active=false;}
  open(...args){this.calls.push(['open',...args]);if(this.openError)throw this.openError;this.active=false;this.responseURL=new URL(args[1],'http://local/').href;return 'opened';}
  send(...args){this.calls.push(['send',...args]);if(this.sendError)throw this.sendError;if(this.active)throw new Error('already sending');this.active=true;return 'sent';}
  getResponseHeader(name){if(this.headerError)throw this.headerError;return this.headers[name]??null;}
  complete(headers){this.headers=headers;this.active=false;this.dispatchEvent(new Event('load'));this.dispatchEvent(new Event('loadend'));}
  abort(){this.active=false;this.dispatchEvent(new Event('abort'));this.dispatchEvent(new Event('loadend'));}
  fail(){this.active=false;this.dispatchEvent(new Event('error'));this.dispatchEvent(new Event('loadend'));}
 }
 let result, thrown;
 const opened=[], timers=new Map(), events=new Map();
 let timerID=0, savedTheme=options.theme;
 const media={matches:!!options.dark,addEventListener(name,handler){events.set('media:'+name,handler);}};
 const window={fetch:options.fetch || (async()=>{if(thrown)throw thrown;return result}),open:(...args)=>{opened.push(args);return {focus(){}}}};
 Object.assign(window,{
  setTimeout(fn,ms){timers.set(++timerID,{fn,ms});return timerID;},
  clearTimeout(id){timers.delete(id);},
  matchMedia(){return media;},
  localStorage:{getItem(){if(options.storageError)throw new Error('storage denied');return savedTheme;}},
  addEventListener(name,handler){events.set(name,handler);}
 });
 const document={currentScript:{dataset:{errors:'0',request:'hb-1'}},readyState:'complete',createElement:element,body:element('body')};
 const context={window,document,XMLHttpRequest:XHR,URL,location:new URL(options.origin || 'http://local/'),Map,WeakMap};
 vm.runInNewContext(readFileSync(new URL('./status-indicator.js',import.meta.url),'utf8'),context);
 return {window,XHR,opened,timers,host:elements.find(e=>e.tag==='hyperbricks-status'),
  theme(value){savedTheme=value;events.get('storage')({key:'hb-theme'});},
  systemDark(value){media.matches=value;events.get('media:change')();},
  tooltip:elements.find(e=>e.tag==='span'),button:elements.find(e=>e.tag==='button'),
  response(count,id,url='http://local/piece'){result={url,headers:new Headers({'X-Hyperbricks-Render-Error-Count':count,'X-Hyperbricks-Request-ID':id})};return result},
  reject(error){thrown=error}};
}
test('fetch preserves response and rejection; cross-origin responses cannot change status',async()=>{
 const h=browserHarness();
 const response=h.response('1','hb-2');
 assert.equal(await h.window.fetch('/piece'),response);
 assert.equal(h.button.dataset.status,'error');
 h.response('0','hb-3','https://external.test/piece');
 await h.window.fetch('https://external.test/piece');
 assert.equal(h.button.dataset.status,'error');
 h.response('0','hb-4');await h.window.fetch('/piece');
 assert.equal(h.button.dataset.status,'ok');
 const failure=new Error('network');h.reject(failure);
 await assert.rejects(h.window.fetch('/piece'),e=>e===failure);
});
test('XHR failure and recovery, and clicks use one named target',()=>{
 const h=browserHarness(), xhr=new h.XHR();
 h.button.listeners.click();
 xhr.open('GET','/piece');xhr.send();
 xhr.complete({'X-Hyperbricks-Render-Error-Count':'1','X-Hyperbricks-Request-ID':'hb-2'});
 assert.equal(h.button.dataset.status,'error');h.button.listeners.click();
 xhr.open('GET','/piece');xhr.send();
 xhr.complete({'X-Hyperbricks-Render-Error-Count':'0','X-Hyperbricks-Request-ID':'hb-3'});
 assert.equal(h.button.dataset.status,'ok');h.button.listeners.click();
 assert.deepEqual(h.opened.map(x=>x[1]),['hyperbricks-devtools','hyperbricks-devtools','hyperbricks-devtools']);
 assert.equal(h.opened[1][0],'/__hyperbricks/errors?request_id=hb-2');
 assert.equal(h.opened[2][0],'/__hyperbricks/dashboard');
});

test('tooltip follows status from success to fragment error and recovery',async()=>{
 const h=browserHarness();
 assert.equal(h.tooltip.getAttribute('data-tip'),'Open HyperBricks Dashboard');
 h.response('1','hb-2');await h.window.fetch('/piece');
 assert.equal(h.tooltip.getAttribute('data-tip'),'Open HyperBricks Diagnostics');
 h.response('0','hb-3');await h.window.fetch('/unrelated');
 assert.equal(h.tooltip.getAttribute('data-tip'),'Open HyperBricks Diagnostics');
 h.response('0','hb-4');await h.window.fetch('/piece');
 assert.equal(h.tooltip.getAttribute('data-tip'),'Open HyperBricks Dashboard');
});

test('fetch forwards receiver and arguments, leaves a real Response body unread despite observer failure',async()=>{
 const body=new ReadableStream({start(controller){controller.enqueue(new TextEncoder().encode('streamed'));controller.close();}});
 const response=new Response(body);
 Object.defineProperty(response,'headers',{get(){throw new Error('indicator header read failed');}});
 let call;
 const h=browserHarness({fetch:function(...args){call={receiver:this,args};return Promise.resolve(response);}});
 const receiver={}, input=new Request('http://local/piece'), init={signal:new AbortController().signal,headers:{custom:'value'}};
 assert.equal(await h.window.fetch.call(receiver,input,init),response);
 assert.equal(call.receiver,receiver);assert.equal(call.args[0],input);assert.equal(call.args[1],init);
 assert.equal(response.bodyUsed,false);assert.equal(await response.text(),'streamed');
});
test('indicator painting failures cannot reject a successful fetch or prevent recovery',async()=>{
 const h=browserHarness(), original=h.tooltip.setAttribute;
 h.tooltip.setAttribute=()=>{throw new Error('paint failed');};
 const response=h.response('1','hb-2');assert.equal(await h.window.fetch('/piece'),response);
 h.tooltip.setAttribute=original;
 h.response('0','hb-3');await h.window.fetch('/piece');
 assert.equal(h.tooltip.getAttribute('data-tip'),'Open HyperBricks Dashboard');
});
test('fetch setup failure still calls underlying fetch; synchronous exceptions retain identity',async()=>{
 let calls=0;const response=new Response('ok');
 const h=browserHarness({fetch:()=>{calls++;return Promise.resolve(response);}});
 const options={get method(){throw new Error('observer setup failed');}};
 assert.equal(await h.window.fetch('/piece',options),response);assert.equal(calls,1);
 const failure=new TypeError('native failure');const other=browserHarness({fetch:()=>{throw failure;}});
 assert.throws(()=>other.window.fetch('/piece'),e=>e===failure);
});
test('fetch abort signal and exact rejection survive observation',async()=>{
 const controller=new AbortController();const reason=new DOMException('Cancelled','AbortError');
 let received;
 const h=browserHarness({fetch:(input,options)=>{
  received=options.signal;
  return new Promise((resolve,reject)=>{
   if(options.signal.aborted)reject(options.signal.reason);
   else options.signal.addEventListener('abort',()=>reject(options.signal.reason),{once:true});
  });
 }});
 const request=h.window.fetch('/piece',{signal:controller.signal});
 controller.abort(reason);
 await assert.rejects(request,e=>e===reason);assert.equal(received,controller.signal);
 await assert.rejects(h.window.fetch('/piece',{signal:controller.signal}),e=>e===reason);
});
test('XHR preserves arguments, returns and application events through header and paint failures',()=>{
 const h=browserHarness(), xhr=new h.XHR(), seen=[];
 for(const type of ['load','loadend'])xhr.addEventListener(type,()=>seen.push(type));
 const body=new Uint8Array([1,2]);
 assert.equal(xhr.open('POST','/piece',true,'user','password'),'opened');
 assert.equal(xhr.send(body),'sent');
 assert.deepEqual(xhr.calls,[['open','POST','/piece',true,'user','password'],['send',body]]);
 xhr.headerError=new Error('header read');
 assert.doesNotThrow(()=>xhr.complete({}));assert.deepEqual(seen,['load','loadend']);
 xhr.headerError=null;h.tooltip.setAttribute=()=>{throw new Error('paint');};
 xhr.open('GET','/piece');xhr.send();
 assert.doesNotThrow(()=>xhr.complete({'X-Hyperbricks-Render-Error-Count':'1','X-Hyperbricks-Request-ID':'hb-2'}));
 assert.deepEqual(seen,['load','loadend','load','loadend']);
});
test('XHR abort and network errors preserve events and do not clear previous render failures; reuse recovers',()=>{
 const h=browserHarness(), xhr=new h.XHR(), seen=[];
 for(const type of ['load','abort','error','loadend'])xhr.addEventListener(type,()=>seen.push(type));
 xhr.open('GET','/piece');xhr.send();xhr.complete({'X-Hyperbricks-Render-Error-Count':'1','X-Hyperbricks-Request-ID':'hb-2'});
 for(const finish of ['abort','fail']){
  xhr.open('GET','/piece');xhr.send();xhr.headers={'X-Hyperbricks-Render-Error-Count':'0','X-Hyperbricks-Request-ID':'hb-3'};
  xhr[finish]();assert.equal(h.button.dataset.status,'error');
 }
 xhr.open('GET','/piece');xhr.send();xhr.complete({'X-Hyperbricks-Render-Error-Count':'0','X-Hyperbricks-Request-ID':'hb-4'});
 assert.equal(h.button.dataset.status,'ok');
 assert.deepEqual(seen,['load','loadend','abort','loadend','error','loadend','load','loadend']);
});
test('XHR native errors retain identity and a subsequent send recovers',()=>{
 const h=browserHarness(), xhr=new h.XHR(), failure=new Error('native failure');
 xhr.openError=failure;assert.throws(()=>xhr.open('GET','/piece'),e=>e===failure);
 xhr.openError=null;xhr.open('GET','/piece');xhr.sendError=failure;
 assert.throws(()=>xhr.send(),e=>e===failure);
 xhr.sendError=null;xhr.send();assert.throws(()=>xhr.send(),/already sending/);
 xhr.complete({'X-Hyperbricks-Render-Error-Count':'1','X-Hyperbricks-Request-ID':'hb-2'});
 assert.equal(h.button.dataset.status,'error');
});
test('XHR observation registration failure does not stop native send',()=>{
 const h=browserHarness(), xhr=new h.XHR();xhr.open('GET','/piece');
 xhr.addEventListener=()=>{throw new Error('listener setup failed');};
 assert.equal(xhr.send('body'),'sent');
 assert.doesNotThrow(()=>xhr.complete({}));
});

test('real fetch cancellation before and during a request preserves AbortError',async()=>{
 const {createServer}=await import('node:http');
 let arrived;
 const arrival=new Promise(resolve=>{arrived=resolve;});
 const server=createServer((req,res)=>{arrived();});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 try {
  const h=browserHarness({fetch,origin:`http://127.0.0.1:${server.address().port}/`});
  const url=`http://127.0.0.1:${server.address().port}/pending`;
  const before=new AbortController();before.abort();
  await assert.rejects(h.window.fetch(url,{signal:before.signal}),error=>error===before.signal.reason);
  const during=new AbortController();
  const pending=h.window.fetch(url,{signal:during.signal});
  const rejection=assert.rejects(pending,error=>error===during.signal.reason);
  await arrival;during.abort();await rejection;
 } finally {
  server.closeAllConnections();
  await new Promise(resolve=>server.close(resolve));
 }
});
test('XHR re-open removes old observers and preserves the new request',()=>{
 const h=browserHarness(), xhr=new h.XHR();
 xhr.open('GET','/old');xhr.send();
 xhr.open('GET','/new');xhr.send();
 xhr.complete({'X-Hyperbricks-Render-Error-Count':'1','X-Hyperbricks-Request-ID':'hb-2'});
 xhr.open('GET','/new');xhr.send();
 xhr.complete({'X-Hyperbricks-Render-Error-Count':'0','X-Hyperbricks-Request-ID':'hb-3'});
 assert.equal(h.button.dataset.status,'ok');
});

test('ok hides after five seconds, errors reappear, recovery hides again without extending on unrelated successes',async()=>{
 const h=browserHarness();
 assert.equal(h.host.style.display,'block');
 assert.equal(h.timers.size,1);
 const first=[...h.timers.values()][0];assert.equal(first.ms,5000);first.fn();
 assert.equal(h.host.style.display,'none');
 h.response('0','hb-2');await h.window.fetch('/unrelated');
 assert.equal(h.host.style.display,'none');
 h.response('1','hb-3');await h.window.fetch('/piece');
 assert.equal(h.host.style.display,'block');assert.equal(h.timers.size,0);
 h.response('0','hb-4');await h.window.fetch('/piece');
 assert.equal(h.host.style.display,'block');
 const timer=[...h.timers.values()][0];
 h.response('0','hb-5');await h.window.fetch('/another');
 assert.equal([...h.timers.values()][0],timer);
 timer.fn();assert.equal(h.host.style.display,'none');
});
test('badge follows dashboard preference and system theme without changing the page',()=>{
 const h=browserHarness({theme:'night'});
 assert.equal(h.tooltip.getAttribute('data-theme'),'night');
 h.theme('light');assert.equal(h.tooltip.getAttribute('data-theme'),'lofi');
 h.systemDark(true);assert.equal(h.tooltip.getAttribute('data-theme'),'lofi');
 h.theme(null);assert.equal(h.tooltip.getAttribute('data-theme'),'night');
 h.systemDark(false);assert.equal(h.tooltip.getAttribute('data-theme'),'lofi');
 const blocked=browserHarness({storageError:true,dark:true});
 assert.equal(blocked.tooltip.getAttribute('data-theme'),'night');
});
