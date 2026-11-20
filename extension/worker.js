import {shouldTakeover} from './decide.js';

const alt = new Map();

chrome.runtime.onMessage.addListener((message, sender) => {
  if (message.type !== 'alt' || !sender.tab) return;
  const old = alt.get(sender.tab.id);
  alt.set(sender.tab.id, {
    held: !!message.held,
    at: Date.now(),
    lastHeld: message.held ? Date.now() : old?.lastHeld || 0
  });
});

chrome.tabs.onRemoved.addListener(id => alt.delete(id));

chrome.downloads.onCreated.addListener(async item => {
  const now = Date.now();
  const skip = [...alt.values()].some(s => s.held || now - s.lastHeld < 3000);
  if (!shouldTakeover(item, skip)) return;
  try {
    await chrome.downloads.pause(item.id);
    const url = item.finalUrl || item.url;
    const cookies = await chrome.cookies.getAll({url});
    const reply = await chrome.runtime.sendNativeMessage('com.jellli.jisu', {
      url,
      referrer: item.referrer || '',
      cookies: cookies.filter(c => !c.partitionKey).map(c => `${c.name}=${c.value}`).join('; '),
      userAgent: navigator.userAgent,
      filename: item.filename
    });
    if (!reply?.ok) throw new Error('Takeover failed');
    await chrome.downloads.cancel(item.id);
    await chrome.downloads.erase({id: item.id});
  } catch {
    await chrome.downloads.resume(item.id).catch(() => {});
  }
});
