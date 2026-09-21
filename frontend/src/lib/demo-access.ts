// Demo access of the public demo (#3462). The token arrives in the URL
// fragment and leaves the browser only in a POST body, so no server or proxy
// log records it.

/** Demo page of the website; the way back for an unknown or expired link. */
export const DEMO_WEBSITE_URL = "https://moto-ogs.de/demo";

/**
 * What a visitor of the public demo sees on the way in.
 * "unavailable": the demo school could not be set up; only a new request helps.
 */
export type DemoEntryPhase =
  "opening" | "preparing" | "invalid" | "failed" | "unavailable";

// The waiting room on the main domain and the entry page of the demo school
// (#3463) say the same thing in the same words.
export const DEMO_ENTRY_LOADING: Record<
  Extract<DemoEntryPhase, "opening" | "preparing">,
  string
> = {
  opening: "Demo wird geöffnet …",
  preparing: "Ihre Demo wird vorbereitet. Einen Moment bitte.",
};

export const DEMO_ENTRY_PROBLEMS: Record<
  Exclude<DemoEntryPhase, "opening" | "preparing">,
  { title: string; description: string }
> = {
  invalid: {
    title: "Dieser Link funktioniert nicht mehr",
    description:
      "Ein Demo-Link gilt 14 Tage. Auf unserer Website bekommen Sie sofort einen neuen.",
  },
  failed: {
    title: "Das hat leider nicht geklappt",
    description:
      "Es liegt nicht an Ihnen. Bitte öffnen Sie den Link noch einmal. Oder fordern Sie einen neuen an.",
  },
  unavailable: {
    title: "Das hat leider nicht geklappt",
    description:
      "Es liegt nicht an Ihnen. Bitte fordern Sie auf unserer Website einen neuen Link an.",
  },
};

/**
 * True for the demo entry page, on a school subdomain (`/demo`) and in path
 * mode (`/{slug}/demo`).
 */
export function isDemoEntryPath(
  pathname: string | null,
  tenantSlug: string | undefined,
): boolean {
  if (!pathname) return false;
  const path = pathname.replace(/\/+$/, "");
  return path === "/demo" || (!!tenantSlug && path === `/${tenantSlug}/demo`);
}

type DemoAccessStatus = "preparing" | "ready" | "failed" | "invalid";

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

interface DemoAccessProgress {
  status: DemoAccessStatus;
  /** Origin of the demo school; present once the status is `ready`. */
  schoolUrl?: string;
}

/** Unknown and expired links are `invalid`; anything else unexpected throws. */
async function fetchDemoAccessProgress(
  token: string,
): Promise<DemoAccessProgress> {
  const response = await postToken("/api/demo/access/status", token);
  if (response.status === 404 || response.status === 410) {
    return { status: "invalid" };
  }
  if (!response.ok) throw new Error(`demo status failed: ${response.status}`);
  const body = (await response.json()) as {
    status?: string;
    school_url?: string;
  };
  if (body.status === "ready") {
    return { status: "ready", schoolUrl: body.school_url };
  }
  return { status: body.status === "failed" ? "failed" : "preparing" };
}

const POLL_INTERVAL_MS = 2000;
// Two minutes; a demo school that is not ready by then will not become so.
const MAX_POLLS = 60;

export type DemoWaitOutcome =
  | { phase: "ready"; schoolUrl?: string }
  | { phase: "invalid" | "failed" | "unavailable" | "cancelled" };

/**
 * Polls until the demo school can be entered (#3463). `failed` is a wait that
 * ran out; `unavailable` is a school that could not be set up at all.
 */
export async function waitForDemoSchool(
  token: string,
  run: { cancelled: boolean },
  onPreparing: () => void,
): Promise<DemoWaitOutcome> {
  for (let attempt = 0; !run.cancelled; attempt++) {
    const progress = await fetchDemoAccessProgress(token);
    if (run.cancelled) break;
    if (progress.status === "invalid") return { phase: "invalid" };
    if (progress.status === "failed") return { phase: "unavailable" };
    if (progress.status === "ready") {
      return { phase: "ready", schoolUrl: progress.schoolUrl };
    }
    if (attempt >= MAX_POLLS) return { phase: "failed" };
    onPreparing();
    await new Promise<void>((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
  }
  return { phase: "cancelled" };
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
