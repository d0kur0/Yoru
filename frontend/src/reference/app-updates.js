export function updatePresentation(state, current) {
 const version=state.version||current;
 switch(state.phase){
 case 'checking':return {text:'Проверяем новые версии…',label:'Проверяем…',disabled:true};
 case 'available':return {text:`Доступна версия ${version}`,label:'Скачать обновление',action:'download'};
 case 'downloading':return {text:state.total>0?`Загрузка ${Math.min(100,Math.floor(state.downloaded/state.total*100))}%`:'Загружаем обновление…',label:'Загрузка…',disabled:true};
 case 'ready':return {text:`Версия ${version} загружена и проверена`,label:state.platform==='darwin'?'Открыть установщик':'Установить и перезапустить',action:'install'};
 case 'installing':return {text:'Открываем установщик…',label:'Установка…',disabled:true};
 case 'current':return {text:'Установлена актуальная версия',label:'Проверить снова',action:'check'};
 case 'error':return {text:state.error||'Не удалось проверить обновления',label:'Повторить',action:'check'};
 default:return {text:'Проверка новых версий на GitHub',label:'Проверить обновления',action:'check'};
 }
}

export function createAppUpdates({api,current,esc,row,showModal,toast,beforeInstall}) {
 let state={phase:'idle'}, pending=false;
 const controls=()=>{
  const view=updatePresentation(state,current);
  const button=`<button class="button ghost" data-app-update="${view.action||'check'}" ${view.disabled||!api?'disabled':''} ${view.disabled?'aria-busy="true"':''}>${esc(view.label)}</button>`;
  const notes=state.version&&state.releaseURL?`<a class="text-link" href="${esc(state.releaseURL)}" target="_blank" rel="noreferrer">Что нового в ${esc(state.version)} ↗</a>`:'';
  return row(`Yoru ${esc(current)}`,`<span role="status">${esc(view.text)}</span>${notes?`<br>${notes}`:''}`,button);
 };
 const render=()=>{const el=document.querySelector('[data-app-updates]');if(el)el.innerHTML=controls();};
 async function run(method){
  if(!api||pending)return;
  pending=true;state={...state,phase:method==='DownloadAppUpdate'?'downloading':'checking',error:''};render();
  const timer=setInterval(async()=>{try{const next=JSON.parse(await api.AppUpdateStatus());if(pending){state=next;render();}}catch{}},700);
  try{state=JSON.parse(await api[method]());}catch(error){state={...state,phase:'error',error:error.message};}
  finally{clearInterval(timer);pending=false;render();}
 }
 document.addEventListener('click',event=>{
  const button=event.target.closest('[data-app-update]');if(!button||pending)return;
  if(button.dataset.appUpdate==='install'){
   const mac=state.platform==='darwin';
   showModal(`Установить Yoru ${esc(state.version)}?`,mac?'Откроется образ установщика.':'Yoru установит обновление и запустится снова.',`<p class="delete-text">Текущее VPN-подключение будет остановлено. ${mac?'Перенесите Yoru в «Программы», подтвердите замену и запустите приложение.':'Установка пройдёт без мастера настройки. Yoru запустится автоматически; подключение при запуске — по вашим настройкам.'} Настройки сохранятся.</p>`,async()=>{await beforeInstall();try{await api.InstallAppUpdate();}catch(error){state=JSON.parse(await api.AppUpdateStatus());render();throw error;}},mac?'Открыть и выйти':'Обновить и перезапустить');
  }else void run(button.dataset.appUpdate==='download'?'DownloadAppUpdate':'CheckAppUpdate');
 });
 return {
  markup:()=>`<div class="group-label">Обновления приложения</div><section class="native-settings" data-app-updates>${controls()}</section>`,
  async start(){if(!api)return;await run('CheckAppUpdate');if(state.phase==='available')toast(`Доступна Yoru ${state.version} · обновление в настройках`);},
 };
}
