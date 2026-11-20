(() => {
  const report = held => chrome.runtime.sendMessage({type: 'alt', held, at: Date.now()}).catch(() => {});
  addEventListener('keydown', e => { if (e.key === 'Alt') report(true); }, true);
  addEventListener('keyup', e => { if (e.key === 'Alt') report(false); }, true);
  addEventListener('blur', () => report(false));
  for (const type of ['mousedown', 'click']) addEventListener(type, e => report(e.altKey), true);
})();
