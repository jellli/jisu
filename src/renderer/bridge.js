(() => {
  const native = window.mygo;
  if (!native) return;

  window.jisu = Object.freeze({
    getState: () => native.call("Jisu.GetState"),
    add: (url) => native.call("Jisu.Add", url),
    pause: (id) => native.call("Jisu.Pause", id),
    resume: (id) => native.call("Jisu.Resume", id),
    remove: (id) => native.call("Jisu.Remove", id),
    reveal: (id) => native.call("Jisu.Reveal", id),
    chooseDirectory: async () => (await native.call("Jisu.ChooseDirectory")) || null,
    setConcurrency: (count) => native.call("Jisu.SetConcurrency", count),
    onState: (callback) => native.on("jisu:state", callback),
  });
})();
