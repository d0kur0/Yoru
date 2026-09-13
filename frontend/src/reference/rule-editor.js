import {pickProcess} from './process-picker.js';
import {ruleTypes,ruleType} from './rule-types.js';

export function openRuleEditor({id,initial,state,getState,showModal,selectField,field,esc,upsert,toast}) {
 const previous=state.rules.find(r=>r.id===id);
 showModal(previous?'Изменить правило':'Добавить правило',initial?'Правило для приложения будет добавлено первым среди отдельных правил. Наборы маршрутизации проверяются раньше.':'Условия проверяются сверху вниз до первого совпадения.',
  selectField('Условие','type',ruleTypes.map(t=>[t.id,t.label]),previous?.type||initial?.type||'DOMAIN-SUFFIX')+
  '<div class="rule-explanation" id="rule-explanation" aria-live="polite"></div>'+
  '<div class="rule-value-row">'+field('Значение','value',previous?.value||initial?.value||'')+
  '<div id="process-tools" hidden><button type="button" class="button" id="pick-process">Выбрать…</button></div></div>'+
  `<label class="rule-no-resolve" id="rule-no-resolve" hidden><input type="checkbox" name="noResolve" ${previous?.noResolve?'checked':''}> Не запрашивать DNS ради проверки IP <small>Проверять уже известный адрес назначения · no-resolve</small></label>`+
  selectField('Маршрут','action',[['PROXY','Через VPN'],['DIRECT','Напрямую'],['REJECT','Блокировать']],previous?.action||initial?.action||'PROXY')+`<div id="rule-vpn-target">${selectField('Через какой сервер','target',[['','Общая группа — режим из настроек серверов'],['@fastest','Минимальная задержка — автоматический выбор'],...state.servers.map(s=>[s.id,s.name+' · '+s.protocol]),...(previous?.target&&previous.target!=='@fastest'&&!state.servers.some(s=>s.id===previous.target)?[[previous.target,'Сервер удалён — соединения блокируются']]:[])],previous?.target||'')}<div id="rule-target-description" class="rule-target-description" aria-live="polite"></div></div>`,
  f=>{const item={id:previous?.id||crypto.randomUUID(),type:f.get('type'),value:f.get('value').trim(),action:f.get('action'),target:f.get('action')==='PROXY'?f.get('target'):'',noResolve:f.get('type').startsWith('IP-CIDR')&&f.has('noResolve')};if(!item.value)throw Error('Укажите значение правила.');if(initial&&!previous)getState().rules.unshift(item);else upsert(getState().rules,item);});
 const form=document.querySelector('#modal-form'),type=form.elements.type,input=form.elements.value;
 form.classList.add('rule-editor-form');
 const targetField=form.querySelector('#rule-vpn-target');const targetVisibility=()=>targetField.hidden=form.elements.action.value!=='PROXY';form.elements.action.addEventListener('change',targetVisibility);targetVisibility();
 const describeTarget=()=>{
  const target=form.elements.target.value;
  const paragraphs=target==='@fastest'
   ? [['Выбор сервера','Автоматически выбирается сервер с наименьшей задержкой до контрольного URL.'],['Что измеряется','Время ответа проверочного адреса. Задержка до других сайтов и сервисов может отличаться.']]
   : target
    ? [['Выбор сервера','Соединения этого правила идут только через указанный сервер. Автоматической замены нет.'],['Если сервер недоступен','Соединение не установится. Если сервер удалён из приложения, трафик правила блокируется.']]
    : [['Выбор сервера','Используется режим общей группы: ручной выбор, резерв, минимальная задержка или балансировка.']];
  form.querySelector('#rule-target-description').innerHTML=paragraphs.map(([title,text])=>`<div class="rule-target-note"><strong>${title}</strong><p>${text}</p></div>`).join('')+(!target||target==='@fastest'?'<div class="rule-target-location"><span>Настройки выбора и проверки</span><span>Серверы <b aria-hidden="true">›</b> Выбор VPN-сервера <b aria-hidden="true">›</b> Настроить</span></div>':'');
 };
 form.elements.target.addEventListener('change',describeTarget);describeTarget();


 let selected=initial?.path?{path:initial.path,name:initial.value}:null,oldType=type.value;
 const explain=()=>{const t=ruleType(type.value);form.querySelector('#rule-explanation').innerHTML=type.value.startsWith('PROCESS-')?`<p>${type.value.includes('WILDCARD')?'Маска: * — любое количество символов, ? — один символ.':type.value.includes('PATH')?'Совпадает только указанный путь к процессу.':'Совпадает точное имя процесса.'}</p>`:`<p>${esc(t.description)}</p><code>${esc(t.example)}</code>`;input.placeholder=t.placeholder;form.querySelector('#process-tools').hidden=!type.value.startsWith('PROCESS-');form.querySelector('#rule-no-resolve').hidden=!type.value.startsWith('IP-CIDR');};
 type.addEventListener('change',()=>{
  if(oldType.startsWith('PROCESS-')&&type.value.startsWith('PROCESS-')){
   if(oldType.includes('PATH')&&!type.value.includes('PATH')){const path=input.value;const name=path.split(/[\\/]/).pop();selected={path,name};input.value=name;}
   else if(!oldType.includes('PATH')&&type.value.includes('PATH')){input.value=selected?.name===input.value?selected.path:'';}
  }
  oldType=type.value;explain();
 });explain();
 form.querySelector('#pick-process').onclick=()=>pickProcess({pathMode:type.value.includes('PATH'),esc,onSelect:p=>{selected=p;input.value=type.value.includes('PATH')?p.path:p.name;input.focus();}});
}
