// @ts-nocheck — this file runs in ServiceWorkerGlobalScope; the project
// tsconfig uses the DOM lib, and a triple-slash webworker lib reference
// would leak worker typings into every other file.
/**
 * Service worker for Web Push (#2003).
 *
 * Receives push payloads from the backend web_push channel and shows them as
 * system notifications. The payload is display-safe by contract (title, body,
 * deep link only — never child data); details load authenticated after the
 * user follows the deep link into the app.
 */

const sw = /** @type {ServiceWorkerGlobalScope} */ (
  /** @type {unknown} */ (self)
);

sw.addEventListener("install", () => {
  void sw.skipWaiting();
});

sw.addEventListener("activate", (event) => {
  event.waitUntil(sw.clients.claim());
});

sw.addEventListener("push", (event) => {
  /** @type {{title?: string, body?: string, deepLink?: string, type?: string}} */
  let data = {};
  try {
    data = event.data ? event.data.json() : {};
  } catch {
    data = {};
  }

  const title = data.title || "moto";
  const options = {
    body: data.body || "",
    icon: "/icons/icon-192x192.png",
    badge: "/icons/icon-192x192.png",
    data: { deepLink: data.deepLink || "/" },
    tag: data.type || "moto-notification",
  };

  event.waitUntil(sw.registration.showNotification(title, options));
});

sw.addEventListener("notificationclick", (event) => {
  event.notification.close();

  /** @type {unknown} */
  const rawUrl = event.notification.data?.deepLink;
  // Defense in depth: only app-relative paths may be opened.
  const url =
    typeof rawUrl === "string" &&
    rawUrl.startsWith("/") &&
    !rawUrl.startsWith("//")
      ? rawUrl
      : "/";

  event.waitUntil(
    sw.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((clientList) => {
        const client = clientList.find((c) => "focus" in c);
        if (client) {
          return client.focus().then((focused) => {
            if (focused && "navigate" in focused) {
              return focused.navigate(url);
            }
            return focused;
          });
        }
        return sw.clients.openWindow(url);
      }),
  );
});
