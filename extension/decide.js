const types = new Set('zip rar 7z tar gz tgz bz2 xz zst iso mp4 mkv avi mov webm flv wmv m4v ts exe msi dmg pkg deb rpm apk appimage msix'.split(' '));
const mimes = new Set(['application/zip', 'application/x-rar-compressed', 'application/vnd.rar', 'application/x-7z-compressed', 'application/x-tar', 'application/gzip', 'application/x-bzip2', 'application/x-xz', 'application/zstd', 'application/x-iso9660-image', 'application/x-msdownload', 'application/x-msi', 'application/vnd.microsoft.portable-executable', 'application/x-apple-diskimage', 'application/vnd.apple.installer+xml', 'application/vnd.debian.binary-package', 'application/x-rpm', 'application/vnd.android.package-archive', 'application/vnd.ms-appx', 'application/vnd.ms-appx.bundle', 'application/x-iso9660-appimage']);
export function shouldTakeover(item, skipAlt = false) {
  const url = item.finalUrl || item.url;
  if (!/^https?:\/\//i.test(url || '') || skipAlt || item.byExtensionId || item.state !== 'in_progress') return false;
  let path;
  try { path = new URL(url).pathname; } catch { return false; }
  const ext = (item.filename || path).split('.').pop().toLowerCase();
  return Math.max(item.totalBytes || 0, item.fileSize || 0) >= 5 * 1024 * 1024 || types.has(ext) || mimes.has(item.mime) || /^video\//.test(item.mime || '');
}
