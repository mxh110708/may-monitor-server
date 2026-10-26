// Retire an earlier monitor navigation worker without deleting user caches.
// Live status, public themes, login and administration always use the network.
globalThis.addEventListener('install', () => globalThis.skipWaiting())
globalThis.addEventListener('activate', event => event.waitUntil(globalThis.clients.claim()))
