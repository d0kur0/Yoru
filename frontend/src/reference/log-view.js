export function createLogView({esc,btn,showModal,field,selectField,getState}){
 let directory='',error='',requested=false,loading=false;

 function paint(){
  const path=document.querySelector('#logs-folder-path');
  if(path)path.textContent=directory||'Путь недоступен';
  const message=document.querySelector('#logs-folder-error');
  if(message){message.textContent=error;message.hidden=!error;}
 }
 async function refresh(){
  if(loading||!window.APP_API?.LogsLocation)return;
  loading=true;requested=true;
  try{directory=await window.APP_API.LogsLocation();error='';}
  catch(e){error=e.message||'Не удалось определить папку логов';}
  finally{loading=false;paint();}
 }

 function page(){
  if(!requested&&window.APP_API?.LogsLocation)queueMicrotask(refresh);
  const enabled=getState().logging?.enabled!==false;
  return `<section class="logs-page"><header class="logs-header"><div><h2>Логи</h2><p>Файлы журнала ядра Mihomo на этом компьютере</p></div><div class="logs-actions">${btn('Настройки','log-settings','settings','ghost')}</div></header><div class="logs-folder"><div class="logs-folder-label">Папка журнала</div><code class="logs-folder-path" id="logs-folder-path">${esc(directory||'Определяем путь…')}</code><p class="logs-folder-note">${enabled?'Сбор логов включён.':'Сбор логов выключен.'} Откройте папку, чтобы посмотреть сохранённые файлы.</p><p class="logs-folder-error" id="logs-folder-error" role="alert" ${error?'':'hidden'}>${esc(error)}</p><div class="logs-folder-actions">${btn('Открыть папку','log-open-folder','folder','primary')}</div></div></section>`;
 }

 function settings(){
  const s=getState().logging||{enabled:true,level:'info',maxSizeMB:5,files:5,days:7};
  showModal('Логи','Лимиты хранения применяются сразу. Уровень ядра — после переподключения.',selectField('Собирать журнал','enabled',[['true','Да'],['false','Нет']],String(s.enabled))+selectField('Уровень','level',['debug','info','warning','error','silent'].map(x=>[x,x]),s.level)+`<div class="field-grid">${field('Размер файла, МБ','maxSizeMB',s.maxSizeMB,'1–50 МБ','number')}${field('Всего файлов','files',s.files,'1–10, включая текущий','number')}</div>`+field('Хранить, дней','days',s.days,'1–30 дней','number'),f=>{getState().logging={enabled:f.get('enabled')==='true',level:f.get('level'),maxSizeMB:Number(f.get('maxSizeMB')),files:Number(f.get('files')),days:Number(f.get('days'))}});
 }
 return {page,settings,refresh};
}
