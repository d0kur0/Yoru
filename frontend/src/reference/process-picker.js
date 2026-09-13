import {uniqueProcesses} from './process-list.js';
import {processIconMarkup,loadProcessIcons} from './process-icons.js';

export function pickProcess({pathMode,esc,onSelect}) {
 const dialog=document.createElement('dialog');dialog.className='process-picker-dialog';
 dialog.innerHTML=`<div class="process-picker-head"><div><h2>Выбрать процесс</h2><p>Выбор подставит ${pathMode?'полный путь':'имя процесса'} в правило.</p></div><button type="button" class="dialog-close" aria-label="Закрыть выбор процесса"><svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.8"><path d="m6 6 12 12M18 6 6 18"/></svg></button></div><div class="process-picker-search"><input type="search" placeholder="Найти приложение или путь" aria-label="Поиск процесса"><button type="button" class="button">Обновить</button></div><div class="process-picker-list" aria-live="polite"></div>`;
 document.body.append(dialog);dialog.showModal();
 let processes=[],generation=0;
 const list=dialog.querySelector('.process-picker-list'),search=dialog.querySelector('input');
 const tooltip=document.createElement('div');tooltip.className='process-path-tooltip';tooltip.setAttribute('popover','manual');tooltip.setAttribute('role','tooltip');tooltip.id='process-full-path';dialog.append(tooltip);
 let timer,owner;
 const hideTip=()=>{clearTimeout(timer);if(tooltip.matches(':popover-open'))tooltip.hidePopover();owner?.removeAttribute('aria-describedby');owner=null;};
 const showTip=row=>{hideTip();const path=row?.dataset.fullPath;if(!path)return;owner=row;timer=setTimeout(()=>{if(!row.isConnected)return;tooltip.textContent=path;tooltip.showPopover();const r=row.getBoundingClientRect();const t=tooltip.getBoundingClientRect();tooltip.style.left=Math.max(12,Math.min(r.left,innerWidth-t.width-12))+'px';tooltip.style.top=Math.max((document.querySelector('.titlebar')?.getBoundingClientRect().bottom||0)+8,r.bottom+t.height+8<innerHeight?r.bottom+6:r.top-t.height-6)+'px';row.setAttribute('aria-describedby',tooltip.id);},300);};
 const draw=()=>{hideTip();const q=search.value.toLowerCase();const rows=processes.filter(p=>`${p.name} ${p.path}`.toLowerCase().includes(q));list.innerHTML=rows.length?rows.map(p=>`<div class="process-picker-item"><button type="button" class="process-picker-row" data-pid="${p.pid}" data-full-path="${esc(p.path||'')}" ${pathMode&&!p.path?'disabled':''}>${processIconMarkup(p.path,esc)}<span class="process-picker-name"><strong>${esc(p.name)}</strong><small>${esc(p.path||'Путь недоступен')}</small></span></button></div>`).join(''):'<p class="empty">Приложения не найдены</p>';void loadProcessIcons(window.APP_API);};
 list.addEventListener('pointerover',e=>{const row=e.target.closest('[data-full-path]');if(row&&!row.contains(e.relatedTarget))showTip(row);});
 list.addEventListener('pointerout',e=>{const row=e.target.closest('[data-full-path]');if(row&&!row.contains(e.relatedTarget))hideTip();});
 list.addEventListener('focusin',e=>showTip(e.target.closest('[data-full-path]')));list.addEventListener('focusout',hideTip);list.addEventListener('scroll',hideTip);search.addEventListener('input',hideTip);
 dialog.addEventListener('keydown',e=>{if(e.key==='Escape'&&tooltip.matches(':popover-open')){e.preventDefault();e.stopPropagation();hideTip();}});
 async function load(){const n=++generation;list.textContent='Загрузка процессов…';try{if(!window.APP_API?.ListProcesses)throw Error('Список доступен в desktop-приложении');const result=JSON.parse(await window.APP_API.ListProcesses());if(dialog.open&&generation===n){processes=uniqueProcesses(result,document.documentElement.dataset.platform==='windows');draw();}}catch(e){if(dialog.open&&generation===n)list.textContent=e.message;}}
 dialog.querySelector('.process-picker-head button').onclick=()=>dialog.close();
 dialog.querySelector('.process-picker-search button').onclick=load;search.oninput=draw;
 list.onclick=e=>{const b=e.target.closest('[data-pid]');if(!b||b.disabled)return;const p=processes.find(p=>String(p.pid)===b.dataset.pid);if(p){onSelect(p);dialog.close();}};
 dialog.addEventListener('close',()=>{hideTip();dialog.remove();},{once:true});search.focus();void load();
}
