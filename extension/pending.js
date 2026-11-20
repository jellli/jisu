import {shouldTakeover} from './decide.js';

export function createTakeoverTracker() {
  const creationAlt = new Map();
  const handled = new Set();

  return {
    created(item, skipAlt) {
      creationAlt.set(item.id, skipAlt);
      return this.claim(item);
    },
    claim(item) {
      if (!creationAlt.has(item.id) || handled.has(item.id)) return false;
      if (!shouldTakeover(item, creationAlt.get(item.id))) return false;
      handled.add(item.id);
      return true;
    }
  };
}
