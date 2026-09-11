// Same-origin cross-tab ping for settings changes. Cross-origin sync
// (operator ↔ tenant) is handled separately via the `tenant_settings_changed`
// SSE event — see use-global-sse.ts.

const CHANNEL_NAME = "phoenix:settings-changed";

// jsdom does not ship BroadcastChannel; guard for tests.
function getChannelCtor():
  (new (name: string) => BroadcastChannel) | undefined {
  if (typeof BroadcastChannel === "undefined") return undefined;
  return BroadcastChannel;
}

export function notifySettingsChanged(): void {
  const Ctor = getChannelCtor();
  if (!Ctor) return;
  const ch = new Ctor(CHANNEL_NAME);
  ch.postMessage(null);
  // Close after sending so the page stays bfcache-eligible.
  ch.close();
}

export function subscribeSettingsChanged(handler: () => void): () => void {
  const Ctor = getChannelCtor();
  if (!Ctor) return () => {};
  const ch = new Ctor(CHANNEL_NAME);
  ch.onmessage = () => handler();
  return () => ch.close();
}
