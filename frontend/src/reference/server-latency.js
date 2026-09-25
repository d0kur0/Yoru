export function createServerLatency(probe, storage, now=Date.now) {
 let results={};
 try { results=JSON.parse(storage?.getItem('server-latency-v1')||'{}'); } catch {}
 let session='', generation=0, batch=null;
 const due=new Map(), activity=new Map(), pending=new Set();
 function sync(running,key,servers,url,onChange){
  // Node definitions stay in memory only; changed nodes invalidate in-flight replies.
  const next=running?JSON.stringify([key,url,servers.map(({latency,...s})=>s)]):'';
  if(next!==session){session=next;generation++;due.clear();activity.clear();batch=null;onChange();}
 }
 function start(servers,url,onChange,all){
  if(!session)return;
  if(batch){
   if(all)for(const s of servers)if(!batch.ids.has(s.id)){batch.ids.add(s.id);batch.queue.push(s);activity.set(s.id,'queued');}
   onChange();return;
  }
  const queue=servers.filter(s=>all||(due.get(s.id)||0)<=now());
  if(!queue.length)return;
  const run={token:generation,queue,ids:new Set(queue.map(s=>s.id)),completed:0};batch=run;
  for(const s of queue)activity.set(s.id,'queued');onChange();
  async function worker(){
   while(run.token===generation&&run.queue.length){
    if(pending.size>=4){await Promise.race(pending);continue;}
    const s=run.queue.shift();activity.set(s.id,'measuring');onChange();
    const job=Promise.resolve().then(()=>probe(s.id)).catch(()=>-1);pending.add(job);
    try {
     const value=await job;
     if(run.token!==generation)continue;
     results[s.id]={value,time:now(),url};
     const ids=new Set(servers.map(s=>s.id));
     results=Object.fromEntries(Object.entries(results).filter(([id])=>ids.has(id)));
     try {storage?.setItem('server-latency-v1',JSON.stringify(results));} catch {}
     due.set(s.id,now()+(value<0?30000:60000));run.completed++;
     activity.delete(s.id);onChange();
    } finally {pending.delete(job);}
   }
  }
  return Promise.all(Array.from({length:4},worker)).finally(()=>{if(batch===run){batch=null;activity.clear();onChange();}});
 }
 return {
  get(id,url){const r=results[id];return r?.url===url&&r.value!==-2?r:null;},
  activity(id){return activity.get(id)||null;},
  get refreshing(){return batch!==null;},
  get progress(){return batch?{total:batch.ids.size,completed:batch.completed}:null;},
  async tick(running,key,servers,url,onChange=()=>{}){sync(running,key,servers,url,onChange);return start(servers,url,onChange,false);},
  async refresh(running,key,servers,url,onChange=()=>{}){sync(running,key,servers,url,onChange);return start(servers,url,onChange,true);}
 };
}
