// One submit owns the fetch and replacement. The backend validates the whole
// response, backs up the previous config and rejects a stale local revision.
export function createConfigSync({api, beforeSync, getRevision, apply}) {
 let pending=false;
 return async source=>{
  if(pending)return false;
  if(!api)throw Error('Откройте desktop-приложение');
  const value=source.trim();
  let url;try{url=new URL(value);}catch{throw Error('Укажите персональный HTTPS URL конфигурации.');}
  if(url.protocol!=='https:'||url.username||url.password||url.hash)throw Error('Укажите HTTPS URL без логина и фрагмента.');
  pending=true;
  try{
   await beforeSync();
   const next=JSON.parse(await api.SyncConfigURL(value,getRevision()));
   apply(next);
   // Already committed by the backend: the modal must not SaveConfig again.
   return false;
  }finally{pending=false;}
 };
}
