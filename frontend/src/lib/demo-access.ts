// Demo access of the public demo (#3462). The token arrives in the URL
// fragment and leaves the browser only in a POST body, so no server or proxy
// log records it.

/** Demo page of the website; the way back for an unknown or expired link. */
export const DEMO_WEBSITE_URL = "https://moto-ogs.de/demo";

/** Get-to-know page of the website behind „Kostenlos starten" (#3467). */
export const DEMO_START_URL = "https://moto-ogs.de/start/?src=demo";

/**
 * True in the image built for the public demo only. Next.js inlines the
 * value at build time, so no other build ever shows the demo banner.
 */
export function isDemoBuild(): boolean {
  return process.env.NEXT_PUBLIC_APP_ENV === "demo";
}

/**
 * What the visitor sees the demo school as (#3467). The visitor has one
 * account; a switch changes its role and issues a new session.
 */
export type DemoRole = "caregiver" | "lead" | "all";

export const DEMO_ROLES: readonly {
  role: DemoRole;
  label: string;
  description: string;
}[] = [
  {
    role: "caregiver",
    label: "Betreuungskraft",
    description: "Kinder an- und abmelden. Die eigene Gruppe im Blick.",
  },
  {
    role: "lead",
    label: "OGS-Leitung",
    description: "Personal, Planung und Anfragen der Eltern.",
  },
  {
    role: "all",
    label: "Alle Funktionen",
    description: "Alles, was moto kann.",
  },
];

function isDemoRole(value: unknown): value is DemoRole {
  return DEMO_ROLES.some((entry) => entry.role === value);
}

export function demoRoleLabel(role: DemoRole): string {
  return DEMO_ROLES.find((entry) => entry.role === role)?.label ?? role;
}

export const DEMO_ROLE_CHOICE_TITLE = "Wie möchten Sie moto ansehen?";
export const DEMO_ROLE_CHOICE_HINT = "Sie können das später oben wechseln.";

/**
 * What a visitor of the public demo sees on the way in.
 * "unavailable": the demo school could not be set up; only a new request helps.
 */
export type DemoEntryPhase =
  "opening" | "preparing" | "invalid" | "failed" | "unavailable";

// The waiting room on the main domain and the entry page of the demo school
// (#3463) say the same thing in the same words.
export const DEMO_ENTRY_OPENING = "Demo wird geöffnet …";

/** Heading of the setup screen; without a name it speaks of "Ihre Demo". */
export function demoSetupTitle(schoolName?: string): string {
  return `Wir richten ${schoolName?.trim() ? schoolName.trim() : "Ihre Demo"} für Sie ein`;
}

// What a seed does, in the visitor's words (#3464). The status only says
// "preparing", so the lines follow the usual duration of a seed: one line per
// three polls, the last one waits for "ready".
export const DEMO_SETUP_STEPS = [
  "Schule anlegen",
  "Kinder und Gruppen eintragen",
  "OGS-Tag starten",
] as const;

export const DEMO_SETUP_HINT = "Das dauert meist weniger als eine Minute.";

/** What a screen reader hears after each setup line. */
export const DEMO_SETUP_LINE_STATE = {
  done: "erledigt",
  running: "läuft",
  next: "folgt",
} as const;

/** The action of the `failed` phase: the page waits once more. */
export const DEMO_ENTRY_RETRY = "Noch einmal versuchen";

/** The action of every other problem: only a new request helps. */
export const DEMO_ENTRY_NEW_LINK = "Neuen Link anfordern";

const POLLS_PER_SETUP_STEP = 3;

/** Index of the setup line that runs after the given number of polls. */
export function demoSetupStep(polls: number): number {
  return Math.min(
    Math.floor(polls / POLLS_PER_SETUP_STEP),
    DEMO_SETUP_STEPS.length - 1,
  );
}

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
    description: "Es liegt nicht an Ihnen. Bitte versuchen Sie es noch einmal.",
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

interface DemoTokenPair {
  access_token: string;
  refresh_token: string;
}

/** What a demo link carries in its fragment. */
export interface DemoLink {
  token: string;
  /** A role chosen on the website or behind a fair QR code (#3467). */
  role?: DemoRole;
}

/**
 * Reads `#token=…&role=…` and removes the fragment from the address bar.
 * An unknown role counts as none: the entry page then asks for one.
 */
export function takeDemoLinkFromFragment(): DemoLink | null {
  const fragment = new URLSearchParams(globalThis.location.hash.slice(1));
  const token = fragment.get("token")?.trim();
  if (fragment.has("token")) {
    globalThis.history.replaceState(
      null,
      "",
      globalThis.location.pathname + globalThis.location.search,
    );
  }
  if (!token) return null;
  const role = fragment.get("role");
  return isDemoRole(role) ? { token, role } : { token };
}

/** The fragment that hands a demo link on to the demo school's entry page. */
export function demoLinkFragment(link: DemoLink): string {
  const fragment = new URLSearchParams({ token: link.token });
  if (link.role) fragment.set("role", link.role);
  return `#${fragment.toString()}`;
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
  /** The OGS name the prospect gave (#3464). */
  schoolName?: string;
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
    school_name?: string;
  };
  if (body.status === "ready") {
    return { status: "ready", schoolUrl: body.school_url };
  }
  return {
    status: body.status === "failed" ? "failed" : "preparing",
    schoolName: body.school_name,
  };
}

const POLL_INTERVAL_MS = 2000;
// Two minutes; a demo school that is not ready by then will not become so.
const MAX_POLLS = 60;

export type DemoWaitOutcome =
  | { phase: "ready"; schoolUrl?: string }
  | { phase: "invalid" | "failed" | "unavailable" | "cancelled" };

/** What the setup screen shows while the school is prepared (#3464). */
export interface DemoSetupProgress {
  schoolName?: string;
  /** Index into DEMO_SETUP_STEPS of the line that runs now. */
  step: number;
}

/**
 * Polls until the demo school can be entered (#3463). `failed` is a wait that
 * ran out; `unavailable` is a school that could not be set up at all.
 */
export async function waitForDemoSchool(
  token: string,
  run: { cancelled: boolean },
  onPreparing: (progress: DemoSetupProgress) => void,
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
    onPreparing({
      schoolName: progress.schoolName,
      step: demoSetupStep(attempt),
    });
    await new Promise<void>((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
  }
  return { phase: "cancelled" };
}

/**
 * A redeemed demo link: the session and what the banner shows and reports.
 * `accessId` names the demo access in the product analytics, never a person.
 */
export interface DemoSession extends DemoTokenPair {
  visit: DemoVisit;
}

async function postSession(body: {
  token?: string;
  role: DemoRole;
}): Promise<DemoSession | null> {
  const response = await fetch("/api/demo/access/sessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (response.status === 404 || response.status === 410) return null;
  if (!response.ok) throw new Error(`demo redeem failed: ${response.status}`);
  const payload = (await response.json()) as DemoTokenPair & {
    demo?: { access_id?: string; role?: string; src?: string };
  };
  return {
    access_token: payload.access_token,
    refresh_token: payload.refresh_token,
    visit: {
      accessId: payload.demo?.access_id ?? "",
      role: isDemoRole(payload.demo?.role) ? payload.demo.role : body.role,
      src: payload.demo?.src,
    },
  };
}

/**
 * Redeems the link in the chosen role. Returns null for an unknown or
 * expired link. The route keeps the token for later role switches in a
 * cookie no script can read.
 */
export async function redeemDemoAccess(
  token: string,
  role: DemoRole,
): Promise<DemoSession | null> {
  return postSession({ token, role });
}

/**
 * Switches the visitor's one account to another role and returns the new
 * session (#3467). Null when the link behind the demo has expired.
 */
export async function switchDemoRole(
  role: DemoRole,
): Promise<DemoSession | null> {
  return postSession({ role });
}

/**
 * The demo visit as the banner knows it. It survives the full reload after
 * an entry or a switch, which report themselves once the banner is mounted:
 * an event captured right before a navigation may never leave the page.
 */
export interface DemoVisit {
  accessId: string;
  role: DemoRole;
  src?: string;
  pending?: "demo_entered" | "demo_role_switched";
}

const DEMO_VISIT_KEY = "moto-demo-visit";

export function saveDemoVisit(visit: DemoVisit): void {
  try {
    globalThis.localStorage.setItem(DEMO_VISIT_KEY, JSON.stringify(visit));
  } catch {
    // Without storage the banner shows the default role and reports nothing.
  }
}

export function readDemoVisit(): DemoVisit | null {
  try {
    const raw = globalThis.localStorage.getItem(DEMO_VISIT_KEY);
    if (!raw) return null;
    const visit = JSON.parse(raw) as Partial<DemoVisit>;
    if (typeof visit.accessId !== "string" || !isDemoRole(visit.role)) {
      return null;
    }
    return {
      accessId: visit.accessId,
      role: visit.role,
      src: typeof visit.src === "string" ? visit.src : undefined,
      pending:
        visit.pending === "demo_entered" ||
        visit.pending === "demo_role_switched"
          ? visit.pending
          : undefined,
    };
  } catch {
    return null;
  }
}
