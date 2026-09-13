const icons = new Map();
let loading = false;

export function processIconMarkup(path, esc) {
 const url = icons.get(path);
 return `<span class="app-icon system-app-icon" data-process-icon="${esc(path||'')}">${url?`<img src="${url}" alt="">`:'<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="4"/><path d="M3 9h18M7 6.5h.01M10 6.5h.01"/></svg>'}</span>`;
}

// Load once per executable, sequentially, independently of connection polling.
export async function loadProcessIcons(api) {
 if (loading || !api?.ProcessIcon) return;
 loading = true;
 try {
  for (;;) {
   const paths = [...document.querySelectorAll('[data-process-icon]')].map(el=>el.dataset.processIcon);
   const path = paths.find(p=>p&&!icons.has(p));
   if (!path || icons.size>=256) break;
   icons.set(path,'');
   try {
    const url=await api.ProcessIcon(path);
    if (/^data:image\/png;base64,[A-Za-z0-9+/=]+$/.test(url)) {
     icons.set(path,url);
     for(const el of document.querySelectorAll('[data-process-icon]')) {
      if(el.dataset.processIcon===path){const img=document.createElement('img');img.src=url;img.alt='';el.replaceChildren(img);}
     }
    }
   } catch { /* Missing or protected executables keep the fallback icon. */ }
  }
 } finally { loading=false; }
}
