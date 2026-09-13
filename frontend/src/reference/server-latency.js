export function createServerLatency(probe, storage, now=Date.now) {
 let results={};
 try { results=JSON.parse(storage?.getItem('server-latency-v1')||'{}'); } catch {}
 let session='', pending=false, generation=0;
 const due=new Map();
 return {
  get(id,url){const r=results[id];return r?.url===url?r:null;},
  invalidate(){due.clear();},
  async tick(running,key,servers,url,onChange=()=>{}){
   const next=running?`${key}:${url}`:'';
   if(next!==session){session=next;generation++;due.clear();}
   if(!next||pending)return;
   const server=servers.find(s=>(due.get(s.id)||0)<=now());
   if(!server)return;
   pending=true;const token=generation;
   try {
    let value;
    try { value=await probe(server.id); } catch { value=-1; }
    if(token!==generation)return;
    results[server.id]={value,time:now(),url};
    const ids=new Set(servers.map(s=>s.id));
    results=Object.fromEntries(Object.entries(results).filter(([id])=>ids.has(id)));
    try { storage?.setItem('server-latency-v1',JSON.stringify(results)); } catch {}
    due.set(server.id,now()+(value<0?30000:60000));
    onChange();
   } finally {pending=false;}
  }
 };
}
