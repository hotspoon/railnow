const CACHE = 'railnow-v10';
const APP_SHELL = [
  '/',
  '/css/output.css?v=20260730-7',
  '/css/station-select.css?v=20260730-7',
  '/js/htmx.min.js?v=20260730-7',
  '/js/app.js?v=20260730-7',
  '/images/snow-white-2-600.png',
];

self.addEventListener('install', (event) => {
  event.waitUntil(caches.open(CACHE).then((cache) => cache.addAll(APP_SHELL)));
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key)))));
  self.clients.claim();
});

// Timetable pages remain network-first. Only the application shell and image
// assets are cached, so route results never accumulate in browser storage.
self.addEventListener('fetch', (event) => {
  if (event.request.method !== 'GET') return;
  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin) return;

  const isAsset = ['/css/', '/js/', '/images/', '/icons/'].some((path) => url.pathname.startsWith(path));
  if (isAsset) {
    event.respondWith(caches.match(event.request).then((cached) => {
      if (cached) return cached;
      return fetch(event.request).then((response) => {
        // Clone before returning the response. Cloning inside the asynchronous
        // cache callback races with the browser consuming the response body.
        if (response.ok) {
          const cacheResponse = response.clone();
          event.waitUntil(
            caches.open(CACHE).then((cache) => cache.put(event.request, cacheResponse)),
          );
        }
        return response;
      });
    }));
    return;
  }

  if (event.request.mode === 'navigate') {
    event.respondWith(fetch(event.request).catch(() => caches.match('/')));
  }
});
