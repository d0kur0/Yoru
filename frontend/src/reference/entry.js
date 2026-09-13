import product from '../../product.json';
import './style.css';
import './desktop.css';
import './operations.css';

window.APP_PRODUCT = product;
const native = Boolean(window._wails || window.chrome?.webview || window.webkit?.messageHandlers?.external);
window.APP_DESKTOP = native;
document.title = product.name;
document.documentElement.dataset.desktop = String(native);
const operationStatus=document.createElement('div');operationStatus.id='operation-status';operationStatus.role='status';operationStatus.textContent='Выполняется…';operationStatus.hidden=true;document.body.append(operationStatus);
const configurePlatform = platform => {
  const mac = platform === 'darwin' || platform === 'mac';
  document.documentElement.dataset.platform = mac ? 'mac' : 'windows';
  const bar = document.querySelector('.titlebar');
  const controls = document.querySelector('.window-actions');
  if(mac) {
    controls.prepend(controls.querySelector('[data-window="close"]'));
    bar.prepend(controls);
    if (!bar.querySelector('.mac-window-title')) {
      const caption = document.createElement('span');
      caption.className = 'mac-window-title'; caption.textContent = product.name; bar.append(caption);
    }
  }
};
configurePlatform(document.documentElement.dataset.platform);
document.querySelector('.brand').setAttribute('aria-label',`${product.name} — статус`);
document.querySelectorAll('.brandmark').forEach(el=>{const image=document.createElement('img');image.src=product.icon;image.alt='';image.width=32;image.height=32;el.replaceChildren(image)});
document.querySelector('#restore-window').lastChild.textContent = `Открыть ${product.name}`;

window.APP_API = native ? await import('../../bindings/github.com/d0kur0/Yoru/vpn') : null;
await import('./app.js');
await import('./controls.js');

if(native) {
  const [{Window}, appearance] = await Promise.all([import('@wailsio/runtime'), import('../../bindings/github.com/d0kur0/Yoru/appearance')]);
  try { configurePlatform((await appearance.Get()).platform); } catch (error) { console.error("Platform detection failed", error); }
  document.addEventListener('click', async event => {
    const control = event.target.closest('[data-window]');
    if (!control) return;
    event.preventDefault(); event.stopImmediatePropagation();
    try {
      if(control.dataset.window === 'close') await Window.Close();
      if(control.dataset.window === 'minimize') await Window.Minimise();
      if(control.dataset.window === 'maximize') await Window.ToggleMaximise();
    } catch { window.dispatchEvent(new CustomEvent('app-error',{detail:'Не удалось изменить состояние окна'})); }
  },true);
}

const titlebar=document.querySelector('.titlebar');
if(titlebar)new ResizeObserver(()=>document.documentElement.style.setProperty('--app-titlebar-bottom',titlebar.getBoundingClientRect().bottom+'px')).observe(titlebar);
