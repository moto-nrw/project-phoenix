// Cross-tab settings cache invalidation.
//
// SWR caches are per-window. When the user toggles a setting in the
// Einstellungen tab, only that tab's `settings-schema` cache is busted.
// Any other tab open at the same origin (Datenverwaltung, Kindersuche,
// the avatar-rendering surfaces) keeps showing stale values until reload.
//
// BroadcastChannel sends a tiny ping over an in-browser pub/sub bus;
// every tab that subscribes via `subscribeSettingsChanged` gets called
// and invalidates its own SWR cache. Same-origin only — there's no
// network hop, no security concern, no payload (we just say "something
// changed", consumers refetch authoritatively from the server).
//
// Cross-origin coverage (operator.<domain> ↔ <slug>.<domain>) is NOT
// handled here — BroadcastChannel cannot cross origins by spec. The
// backend's `tenant_settings_changed` SSE event covers that path: every
// authenticated tab holds an SSE connection, the global handler in
// hooks/use-global-sse.ts turns the event into a window CustomEvent, and
// TenantProvider re-resolves the tenant on receipt. See TenantProvider
// for the full three-signal subscription (BroadcastChannel + SSE-driven
// CustomEvent + visibilitychange fallback).

const CHANNEL_NAME = "phoenix:settings-changed";

/** Test-friendly accessor — vitest's jsdom doesn't ship BroadcastChannel by default. */
function getChannelCtor():
  | (new (name: string) => BroadcastChannel)
  | undefined {
  if (typeof BroadcastChannel === "undefined") return undefined;
  return BroadcastChannel;
}

/** Notify every other tab that a setting was saved or reset. No payload. */
export function notifySettingsChanged(): void {
  const Ctor = getChannelCtor();
  if (!Ctor) return;
  const ch = new Ctor(CHANNEL_NAME);
  // postMessage(null) is enough — receivers re-fetch authoritatively.
  ch.postMessage(null);
  // Close the channel right after; long-lived senders are unnecessary
  // and prevent the page from being eligible for back/forward cache.
  ch.close();
}

/**
 * Subscribe a callback to "any setting changed in any tab" events.
 * Returns an unsubscribe function for useEffect cleanup.
 */
export function subscribeSettingsChanged(handler: () => void): () => void {
  const Ctor = getChannelCtor();
  if (!Ctor) return () => {};
  const ch = new Ctor(CHANNEL_NAME);
  ch.onmessage = () => handler();
  return () => ch.close();
}
