import { OverlayScrollbars, ClickScrollPlugin } from 'overlayscrollbars';

OverlayScrollbars.plugin(ClickScrollPlugin);

const options = {
  overflow: { x: 'hidden', y: 'scroll' },
  scrollbars: { theme: 'os-theme-yoru', autoHide: 'leave', autoHideDelay: 700, clickScroll: true },
};
const main = document.querySelector('#main');
// Keep the existing viewport: page rendering replaces its contents, not its scrollbars.
if (main) OverlayScrollbars({
  target: main,
  scrollbars: { slot: main.closest('.app') },
  elements: { viewport: main, padding: false, content: false },
}, options);

const instances = new Map();
const selector = 'dialog[open], .process-picker-list, .log-content, .table-scroll, .list-values';
let frame;
function sync() {
  frame = null;
  for (const [element, instance] of instances) {
    if (!element.isConnected || !element.matches(selector) || !instance.elements().scrollbarVertical.scrollbar.isConnected) {
      instance.destroy();
      instances.delete(element);
    }
  }
  for (const element of document.querySelectorAll(selector)) {
    if (instances.has(element)) continue;
    instances.set(element, OverlayScrollbars({
      target: element,
      elements: { viewport: element, padding: false, content: false },
    }, { ...options, overflow: { x: 'scroll', y: 'scroll' } }));
  }
}
new MutationObserver(() => {
  if (!frame) frame = requestAnimationFrame(sync);
}).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['open'] });
sync();
