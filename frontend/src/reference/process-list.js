export function uniqueProcesses(items,windows=false) {
 const normal=v=>windows?String(v||'').toLowerCase():String(v||'');
 const knownNames=new Set(items.filter(p=>p.path).map(p=>normal(p.name)));
 const unique=new Map();
 for(const p of items){
  // A pathless instance adds no useful choice when that executable is already identified.
  if(!p.path&&knownNames.has(normal(p.name)))continue;
  const key=p.path?'path:'+normal(p.path):'name:'+normal(p.name);
  if(!unique.has(key))unique.set(key,p);
 }
 return [...unique.values()].sort((a,b)=>a.name.localeCompare(b.name)||(a.path||'').localeCompare(b.path||''));
}
