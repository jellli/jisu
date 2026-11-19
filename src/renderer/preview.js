// A browser-only design fixture. The native app always uses its real bridge.
(() => {
  if (window.mygo || new URLSearchParams(location.search).get("preview") !== "porcelain") return;
  const MiB = 1024 ** 2;
  const GiB = 1024 ** 3;
  const file = (id, name, host, state, total, ratio, speed = 0, connections = 0) => ({
    id, name, url: `https://${host}/${name}`, state, total,
    received: Math.round(total * ratio), bytesPerSecond: speed * MiB, connections,
    directory: "~/Downloads",
  });
  const state = {
    downloadDirectory: "~/Downloads", concurrency: 2,
    jobs: [
      file("1", "blender-4.3.2-macos-arm64.dmg", "download.blender.org", "downloading", 368 * MiB, .68, 18.6, 4),
      file("2", "Big_Buck_Bunny_4K.mp4", "download.blender.org", "downloading", 1.6 * GiB, .34, 12.8, 4),
      file("3", "ubuntu-24.04.1-desktop-amd64.iso", "releases.ubuntu.com", "queued", 5.8 * GiB, 0),
      file("4", "Design_Resources_2026.zip", "files.example.com", "queued", 248 * MiB, 0),
      file("5", "The_Creative_Independent.pdf", "files.example.com", "completed", 12.4 * MiB, 1),
      file("6", "Inter-4.1.zip", "github.com", "completed", 28.6 * MiB, 1),
    ],
  };
  const listeners = new Set();
  const emit = () => listeners.forEach((listener) => listener(structuredClone(state)));
  window.jisu = Object.freeze({
    getState: async () => structuredClone(state),
    onState: (listener) => { listeners.add(listener); return () => listeners.delete(listener); },
    add: async (url) => {
      const parsed = new URL(url);
      state.jobs.push(file(String(Date.now()), decodeURIComponent(parsed.pathname.split("/").pop()) || "download", parsed.host, "queued", 0, 0));
      emit();
    },
    pause: async (id) => { const job = state.jobs.find((item) => item.id === id); if (job) job.state = "paused"; emit(); },
    resume: async (id) => { const job = state.jobs.find((item) => item.id === id); if (job) job.state = "queued"; emit(); },
    remove: async (id) => { state.jobs = state.jobs.filter((item) => item.id !== id); emit(); },
    reveal: async () => {},
    chooseDirectory: async () => null,
    setConcurrency: async (count) => { state.concurrency = count; emit(); },
  });
  document.getElementById("preview-label").hidden = false;
})();
