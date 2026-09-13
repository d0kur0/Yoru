const eye='<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/></svg>';
const copy='<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="8" y="8" width="12" height="12" rx="2"/><path d="M15 8V4H4v11h4"/></svg>';

export function editServer(options) {
 const {server,draft=server,getState,showModal,field,selectField,esc,upsert}=options;
 const source=getState().subscriptions.find(s=>s.id===server.subscription);
 const secretField=(label,name,value)=>`<div class="server-secret"><label class="field"><span>${label}</span><div class="server-secret-input"><input name="${name}" type="password" autocomplete="off" spellcheck="false" value="${esc(value||'')}"><button type="button" data-reveal="${name}" aria-label="Показать ${label}" title="Показать" aria-pressed="false">${eye}</button><button type="button" data-copy-secret="${name}" aria-label="Копировать ${label}" title="Копировать">${copy}</button></div></label></div>`;
 const section=(title,body)=>`<section class="server-editor-section"><h3>${title}</h3><div class="server-editor-grid">${body}</div></section>`;
 let transportFields='';
 if(draft.protocol==='VLESS'||draft.protocol==='Trojan') {
  transportFields+=field('Имя TLS-сервера · SNI','sni',draft.sni||'','Например, server.example','text',false);
  if(draft.transport==='Reality')transportFields+=secretField('Публичный ключ Reality','publicKey',draft.publicKey)+secretField('Short ID','shortId',draft.shortId);
  if(draft.protocol==='VLESS')transportFields+=field('Flow','flow',draft.flow||'','','text',false)+field('TLS fingerprint','fingerprint',draft.fingerprint||'chrome','','text',false);
  if(draft.transport==='WebSocket')transportFields+=field('WebSocket path','path',draft.path||'/','','text',false)+field('WebSocket Host','wsHost',draft.wsHost||'','','text',false);
 } else if(['Hysteria2','AnyTLS','TUIC'].includes(draft.protocol))transportFields+=field('Имя TLS-сервера · SNI','sni',draft.sni||'','','text',false);
 if(draft.protocol==='Shadowsocks')transportFields+=field('Метод шифрования','cipher',draft.cipher||'aes-128-gcm','','text',false);
 const label=['VLESS','VMess'].includes(draft.protocol)?'UUID':draft.protocol==='WireGuard'?'Приватный ключ':'Пароль';
 const notice=server.subscription?`<aside class="server-source-notice"><div><strong>Подписка: ${esc(source?.name||'Источник недоступен')}</strong><p>Обновление подписки заменит ручные изменения. «Загрузить из подписки» вернёт исходные значения в форму; для применения нажмите «Сохранить».</p></div><button type="button" class="button" id="restore-subscription-server" ${!source?'disabled':''}>Загрузить из подписки</button><label class="set-toggle"><span><strong>Сохранить независимую копию</strong><small>Копия не получает обновления подписки. Оригинал сохраняется.</small></span><input type="checkbox" role="switch" name="independent"><span class="set-toggle-track" aria-hidden="true"></span></label></aside>`:'';
 showModal('Редактировать сервер','Изменения подключения применятся после перезапуска ядра.',`<div class="server-editor">${notice}<div class="server-editor-identity">${field('Название','name',draft.name)}<div class="field server-editor-protocol-field"><span>Протокол</span><div class="server-editor-protocol">${esc(draft.protocol)}<small>${esc(draft.transport==='Imported'?'Импорт':draft.transport)}</small></div></div></div>${section('Подключение',field('Адрес сервера','host',draft.host,'Домен или IP без https://')+field('Порт','port',draft.port,'','number'))}${section('Авторизация',secretField(label,'secret',draft.secret))}${transportFields?section('Транспорт и TLS',transportFields):''}<p class="server-editor-note">Дополнительные импортированные параметры сохраняются. Протокол и тип транспорта этой формы фиксированы; для другого типа подключения добавьте новый сервер.</p><span id="server-editor-feedback" role="status" aria-live="polite"></span></div>`,f=>{
  const item=serverFromForm(server,draft,f);
  upsert(getState().servers,item);
 });
 const form=document.querySelector('#modal-form'),feedback=form.querySelector('#server-editor-feedback');
 form.querySelectorAll('[data-reveal]').forEach(b=>b.onclick=()=>{const input=form.elements[b.dataset.reveal],show=input.type==='password';input.type=show?'text':'password';b.setAttribute('aria-pressed',String(show));b.setAttribute('aria-label',show?'Скрыть значение':'Показать значение');b.title=show?'Скрыть':'Показать';});
 form.querySelectorAll('[data-copy-secret]').forEach(b=>b.onclick=async()=>{try{const ok=await window.APP_API.CopyLogText(form.elements[b.dataset.copySecret].value);feedback.textContent=ok?'Скопировано':'Не удалось скопировать';}catch{feedback.textContent='Не удалось скопировать';}});
 const restore=form.querySelector('#restore-subscription-server');
 if(restore)restore.onclick=async()=>{
  const buttons=[...form.querySelectorAll('button')];buttons.forEach(b=>b.disabled=true);restore.textContent='Загрузка…';
  try {
   const nodes=JSON.parse(await window.APP_API.FetchSubscription(JSON.stringify(source)));
   const original=nodes.find(s=>s.id===server.id);
   if(!original)throw Error('Сервер больше не найден в подписке. Поля формы сохранены. Обновите подписку в разделе «Серверы».');
   if(form.isConnected)editServer({...options,draft:original});
  }catch(e){if(form.isConnected)form.querySelector('#form-error').textContent=e.message;}
  finally{if(form.isConnected){buttons.forEach(b=>b.disabled=false);restore.textContent='Загрузить из подписки';}}
 };
}

export function serverFromForm(server,draft,f) {
  const item=structuredClone(draft);
  for(const key of ['name','host','sni','publicKey','shortId','flow','fingerprint','path','wsHost','cipher'])if(f.has(key))item[key]=f.get(key).trim();
  item.secret=f.get('secret');item.port=Number(f.get('port'));item.latency=0;
  if(item.options){if(f.has('sni')){delete item.options.sni;delete item.options.servername;}if(f.has('flow'))delete item.options.flow;if(f.has('fingerprint'))delete item.options['client-fingerprint'];}
  if(!item.name||!item.host||/[\s/]/.test(item.host))throw Error('Укажите название и адрес сервера без протокола и пути.');
  if(!Number.isInteger(item.port)||item.port<1||item.port>65535)throw Error('Порт должен быть от 1 до 65535.');
  if(!item.secret.trim())throw Error('Заполните поле авторизации.');
  item.id=server.id;
  if(f.has('independent')){item.id=crypto.randomUUID();delete item.subscription;item.name+=' · копия';}
  return item;
}
