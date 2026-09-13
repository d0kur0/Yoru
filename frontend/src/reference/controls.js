// Keep native selects as the source of truth for forms and existing change handlers.
(() => {
  let openSelect = null, counter = 0, typed = '', typedAt = 0;
  const chevron = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="m5 6.5 3 3 3-3"/></svg>';
  const check = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="m4 8 2.5 2.5L12 5"/></svg>';
  const selectorKey = select => select.name || select.getAttribute('aria-label') || Object.keys(select.dataset).find(k => k !== 'enhanced') || '';

  function closeMenu(restoreFocus = false) {
    if (!openSelect) return;
    const current = openSelect;
    openSelect = null;
    if (current.menu.hidePopover && current.menu.matches(':popover-open')) current.menu.hidePopover();
    current.menu.remove();
    current.button.setAttribute('aria-expanded', 'false');
    current.button.removeAttribute('aria-activedescendant');
    current.button.removeAttribute('aria-controls');
    current.button.classList.remove('is-open');
    if (restoreFocus && current.button.isConnected) current.button.focus({preventScroll:true});
  }

  function setActive(index) {
    if (!openSelect) return;
    const options = openSelect.options;
    const next = Math.max(0, Math.min(index, options.length - 1));
    if (next < 0 || options[next]?.disabled) return;
    openSelect.active = next;
    openSelect.menu.querySelectorAll('[role="option"]').forEach((node, i) => node.classList.toggle('is-active', i === next));
    const item = openSelect.menu.children[next];
    openSelect.button.setAttribute('aria-activedescendant', item.id);
    item.scrollIntoView({block:'nearest'});
  }

  function commit(index) {
    if (!openSelect || openSelect.options[index]?.disabled) return;
    const {select, button, options} = openSelect;
    const key = selectorKey(select);
    const nextValue = options[index].value;
    closeMenu(true);
    if (select.value === nextValue) return;
    select.value = nextValue;
    button.querySelector('.select-value').textContent = select.selectedOptions[0]?.textContent || '';
    select.dispatchEvent(new Event('input', {bubbles:true}));
    select.dispatchEvent(new Event('change', {bubbles:true}));
    queueMicrotask(() => {
      enhance();
      if (!button.isConnected) {
        const replacement = [...document.querySelectorAll('select')].find(s => selectorKey(s) === key);
        replacement?.parentElement.querySelector('.select-trigger')?.focus({preventScroll:true});
      }
    });
  }

  function openMenu(select, button) {
    if (openSelect?.button === button) { closeMenu(); return; }
    closeMenu();
    if (select.disabled || !select.options.length) return;
    const menu = document.createElement('div');
    menu.className = 'select-menu';
    menu.id = `select-menu-${++counter}`;
    menu.setAttribute('role','listbox');
    menu.setAttribute('aria-label', button.getAttribute('aria-label'));
    menu.setAttribute('popover','manual');
    const options = [...select.options];
    for (const [index, option] of options.entries()) {
      const item = document.createElement('div');
      item.className = 'select-option';
      item.id = `${menu.id}-${index}`;
      item.setAttribute('role','option');
      item.setAttribute('aria-selected', String(option.selected));
      if (option.disabled) item.setAttribute('aria-disabled','true');
      const text = document.createElement('span');
      text.textContent = option.textContent;
      item.append(text);
      item.insertAdjacentHTML('beforeend',check);
      item.addEventListener('pointermove', () => { if (openSelect?.active !== index) setActive(index); });
      item.addEventListener('click', event => { event.preventDefault(); event.stopPropagation(); commit(index); });
      menu.append(item);
    }
    menu.addEventListener('pointerdown', e => e.preventDefault());
    (select.closest('dialog') || document.body).append(menu);
    openSelect = {select, button, menu, options, active:Math.max(0, select.selectedIndex)};
    button.setAttribute('aria-expanded','true');
    button.setAttribute('aria-controls',menu.id);
    button.classList.add('is-open');
    if (menu.showPopover) menu.showPopover();
    const rect = button.getBoundingClientRect();
    const width = Math.min(Math.max(rect.width, 210), window.innerWidth - 24);
    menu.style.width = `${width}px`;
    const availableBelow = window.innerHeight - rect.bottom - 18;
    const availableAbove = rect.top - 18;
    const wantedHeight = Math.min(options.length * 37 + 12, 270);
    const above = availableBelow < wantedHeight && availableAbove > availableBelow;
    menu.style.maxHeight = `${Math.max(40, Math.min(270, above ? availableAbove : availableBelow))}px`;
    menu.style.left = `${Math.max(12, Math.min(rect.left, window.innerWidth - width - 12))}px`;
    menu.style.top = `${above ? Math.max(12, rect.top - menu.offsetHeight - 6) : rect.bottom + 6}px`;
    setActive(Math.max(0, select.selectedIndex));
  }

  function enhance() {
    if (openSelect && !openSelect.select.isConnected) closeMenu();
    document.querySelectorAll('select:not([data-enhanced])').forEach(select => {
      select.dataset.enhanced = 'true';
      select.hidden = true;
      const wrapper = document.createElement('span');
      wrapper.className = `custom-select ${select.classList.contains('route-select') ? 'select-compact' : 'select-field'}`;
      select.before(wrapper);
      wrapper.append(select);
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'select-trigger';
      button.setAttribute('role','combobox');
      button.setAttribute('aria-haspopup','listbox');
      button.setAttribute('aria-expanded','false');
      button.setAttribute('aria-label',select.getAttribute('aria-label') || select.closest('label')?.querySelector('span')?.textContent || select.name || 'Выбрать значение');
      button.disabled = select.disabled;
      const value = document.createElement('span');
      value.className = 'select-value';
      value.textContent = select.selectedOptions[0]?.textContent || '';
      button.append(value);
      button.insertAdjacentHTML('beforeend',chevron);
      wrapper.append(button);
      button.addEventListener('click', event => { event.preventDefault(); openMenu(select, button); });
      button.addEventListener('blur', () => { if (openSelect?.button === button) closeMenu(); });
      button.addEventListener('keydown', event => {
        const isOpen = openSelect?.button === button;
        if (event.key === 'Escape' && isOpen) { event.preventDefault(); event.stopPropagation(); closeMenu(true); return; }
        if (event.key === 'Tab') { closeMenu(); return; }
        if (['Enter',' ','ArrowDown','ArrowUp','Home','End'].includes(event.key)) {
          event.preventDefault();
          if ((event.key === 'Enter' || event.key === ' ') && isOpen) { commit(openSelect.active); return; }
          if (!isOpen) openMenu(select, button);
          if (!openSelect) return;
          if (event.key === 'Home') setActive(0);
          if (event.key === 'End') setActive(openSelect.options.length - 1);
          if (isOpen && event.key.startsWith('Arrow')) {
            const step = event.key === 'ArrowDown' ? 1 : -1;
            let index = openSelect.active + step;
            while (openSelect.options[index]?.disabled) index += step;
            if (index >= 0 && index < openSelect.options.length) setActive(index);
          }
        } else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
          event.preventDefault();
          typed = Date.now() - typedAt > 700 ? event.key : typed + event.key;
          typedAt = Date.now();
          if (!isOpen) openMenu(select, button);
          const index = openSelect?.options.findIndex(o => !o.disabled && o.textContent.toLocaleLowerCase().startsWith(typed.toLocaleLowerCase()));
          if (index >= 0) setActive(index);
        }
      });
      select.addEventListener('change', () => { value.textContent = select.selectedOptions[0]?.textContent || ''; });
    });
  }
  document.addEventListener('pointerdown', event => {
    if (openSelect && !openSelect.menu.contains(event.target) && !openSelect.button.contains(event.target)) closeMenu();
  },true);
  document.addEventListener('scroll', event => {
    if (openSelect && event.target !== openSelect.menu && !openSelect.menu.contains(event.target)) closeMenu();
  },true);
  document.addEventListener('close', () => closeMenu(),true);
  window.addEventListener('resize', () => closeMenu());
  const observer = new MutationObserver(enhance);
  observer.observe(document.querySelector('#main'), {childList:true,subtree:true});
  observer.observe(document.querySelector('#modal-body'), {childList:true,subtree:true});
  enhance();
})();
