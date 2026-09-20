// Demo access of the public demo (#3462). The token arrives in the URL
// fragment and leaves the browser only in a POST body, so no server or proxy
// log records it.

/** Demo page of the website; the way back for an unknown or expired link. */
export const DEMO_WEBSITE_URL = "https://moto-ogs.de/demo";

export type DemoAccessStatus = "preparing" | "ready" | "invalid";

export interface DemoTokenPair {
  access_token: string;
  refresh_token: string;
}

/** Reads `#token=…` and removes the fragment from the address bar. */
export function takeDemoTokenFromFragment(): string | null {
  const token = new URLSearchParams(globalThis.location.hash.slice(1)).get(
    "token",
  );
  if (token) {
    globalThis.history.replaceState(
      null,
      "",
      globalThis.location.pathname + globalThis.location.search,
    );
  }
  return token?.trim() ? token.trim() : null;
}

async function postToken(path: string, token: string): Promise<Response> {
  return fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token }),
  });
}

/** Unknown and expired links are `invalid`; anything else unexpected throws. */
export async function fetchDemoAccessStatus(
  token: string,
): Promise<DemoAccessStatus> {
  const response = await postToken("/api/demo/access/status", token);
  if (response.status === 404 || response.status === 410) return "invalid";
  if (!response.ok) throw new Error(`demo status failed: ${response.status}`);
  const body = (await response.json()) as { status?: string };
  return body.status === "ready" ? "ready" : "preparing";
}

/** Returns null for an unknown or expired link. */
export async function redeemDemoAccess(
  token: string,
): Promise<DemoTokenPair | null> {
  const response = await postToken("/api/demo/access/sessions", token);
  if (response.status === 404 || response.status === 410) return null;
  if (!response.ok) throw new Error(`demo redeem failed: ${response.status}`);
  return (await response.json()) as DemoTokenPair;
}
