export function createServerLatency(probe, storage, now=Date.now) {
 let results={};
 try { results=JSON.parse(storage?.getItem('server-latency-v1')||'{}'); } catch {}
 let session='', generation=0, pending=null, batch=null;
 const due=new Map(), activity=new Map();
 function sync(running,key,url,onChange){
  const next=running?`${key}:${url}`:'';
  if(next!==session){session=next;generation++;due.clear();activity.clear();batch=null;onChange();}
  return generation;
 }
 async function measure(server,servers,url,token,onChange){
  activity.set(server.id,'measuring');onChange();
  const job={id:server.id,token};pending=job;
  job.promise=(async()=>{
   let value;
   try { value=await probe(server.id); } catch { value=-1; }
   if(token!==generation)return;
   results[server.id]={value,time:now(),url};
   const ids=new Set(servers.map(s=>s.id));
   results=Object.fromEntries(Object.entries(results).filter(([id])=>ids.has(id)));
   try { storage?.setItem('server-latency-v1',JSON.stringify(results)); } catch {}
   due.set(server.id,now()+(value<0?30000:60000));
  })();
  try {await job.promise;} finally {
   if(pending===job)pending=null;
   if(token===generation){activity.delete(server.id);onChange();}
  }
 }
 return {
  get(id,url){const r=results[id];return r?.url===url&&r.value!==-2?r:null;},
  activity(id){return activity.get(id)||null;},
  get refreshing(){return batch!==null;},
  async tick(running,key,servers,url,onChange=()=>{}){
   const token=sync(running,key,url,onChange);
   if(!session||pending||batch)return;
   const server=servers.find(s=>(due.get(s.id)||0)<=now());
   if(server)await measure(server,servers,url,token,onChange);
  },
  async refresh(running,key,servers,url,onChange=()=>{}){
   const token=sync(running,key,url,onChange);
   if(!session||batch)return;
   const run={token};batch=run;
   const existing=pending;
   for(const server of servers)activity.set(server.id,existing?.token===token&&existing.id===server.id?'measuring':'queued');
   onChange();
   try {
    if(existing)await existing.promise;
    for(const server of servers){
     if(token!==generation)return;
     // A measurement already in progress is fresh enough for this refresh.
     if(existing?.token===token&&existing.id===server.id)continue;
     await measure(server,servers,url,token,onChange);
    }
   } finally {
    if(batch===run){batch=null;activity.clear();onChange();}
   }
  }
 };
}
