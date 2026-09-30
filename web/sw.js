const CACHE_NAME = 'easy-notes-shell-v1';
const APP_SHELL = [
    '/editor',
    '/login',
    '/favicon.ico',
    '/manifest.webmanifest',
    '/assets/css/editor.css',
    '/assets/css/login.css',
    '/assets/js/editor.js',
    '/assets/js/login.js'
];

self.addEventListener('install', (event) => {
    event.waitUntil(caches.open(CACHE_NAME).then((cache) => cache.addAll(APP_SHELL)));
    self.skipWaiting();
});

self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((keys) => Promise.all(
            keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))
        ))
    );
    self.clients.claim();
});

// 优先请求服务器以获取新版本，断网时回退到最后一份缓存。
self.addEventListener('fetch', (event) => {
    if (event.request.method !== 'GET' || new URL(event.request.url).origin !== self.location.origin) return;
    if (new URL(event.request.url).pathname.startsWith('/api/')) return;

    event.respondWith(
        fetch(event.request).then((response) => {
            if (response.ok) {
                const copy = response.clone();
                caches.open(CACHE_NAME).then((cache) => cache.put(event.request, copy));
            }
            return response;
        }).catch(() => caches.match(event.request))
    );
});
