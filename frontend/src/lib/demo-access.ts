// Demo access of the public demo (#3462). The token arrives in the URL
// fragment and leaves the browser only in a POST body, so no server or proxy
// log records it.

import { signIn } from "next-auth/react";
import { parentsPortalUrl } from "~/lib/parent-url";

/** Demo page of the website; the way back for an unknown or expired link. */
export const DEMO_WEBSITE_URL = "https://moto-ogs.de/demo";

/** Get-to-know page of the website behind „Kostenlos starten" (#3467). */
export const DEMO_START_URL = "https://moto-ogs.de/start/?src=demo";

/** Privacy policy of the website; it describes the demo recording (#3603). */
export const DEMO_PRIVACY_URL = "https://moto-ogs.de/datenschutz";

// Every demo session is recorded (#3603). The visitor reads that on every
// screen of the way in, before the first click inside the demo school.
export const DEMO_RECORDING_NOTICE =
  "Um moto zu verbessern, werten wir aus, wie die Demo genutzt wird. Was Sie eintippen, sehen wir dabei nicht.";
export const DEMO_RECORDING_PRIVACY_LINK = "Mehr zum Datenschutz";

/**
 * True in the image built for the public demo only. Next.js inlines the
 * value at build time, so no other build ever shows the demo banner.
 */
export function isDemoBuild(): boolean {
  return process.env.NEXT_PUBLIC_APP_ENV === "demo";
}

/**
 * What the visitor sees the demo school as (#3467). The visitor has one
 * account; a switch changes its role and issues a new session. The role
 * parent (#3468) opens the parents app on its own host with the same link.
 */
export type DemoRole = "caregiver" | "lead" | "parent" | "all";

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
    role: "parent",
    label: "Elternteil",
    description: "Die Eltern-App: Kind, Abholung, Krankmeldung, Nachrichten.",
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

/** The role parent lives in the parents app, every other role in the OGS app. */
export function isParentDemoRole(role: DemoRole): boolean {
  return role === "parent";
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
// "preparing", so the lines follow the usual duration of a seed of about
// 30 to 40 seconds by the time waited; the last one waits for "ready".
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

/** When each setup line starts, in milliseconds after the wait began. */
const DEMO_SETUP_STEP_STARTS_MS = [0, 10_000, 25_000] as const;

/** Index of the setup line that runs after the given time of waiting. */
export function demoSetupStep(elapsedMs: number): number {
  const started = DEMO_SETUP_STEP_STARTS_MS.filter(
    (start) => elapsedMs >= start,
  ).length;
  // A clock set back while waiting keeps the first line.
  return Math.max(started - 1, 0);
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
    description: "Bitte versuchen Sie es gleich noch einmal.",
  },
  unavailable: {
    title: "Das hat leider nicht geklappt",
    description:
      "Die Demo konnte nicht vorbereitet werden. Auf unserer Website bekommen Sie sofort einen neuen Link.",
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
  /**
   * The banner sent the visitor over from the other app (#3468): the entry
   * reports a role switch, not a new entry.
   */
  switched?: boolean;
  /**
   * The visitor started the demo over (#3470): the entry into the fresh
   * school reports a restart, not a new entry.
   */
  restarted?: boolean;
}

/** What the entry into a school reports once its start page has loaded. */
export function demoEntryEvent(link: DemoLink): DemoEntryEvent {
  if (link.restarted) return "demo_restarted";
  return link.switched ? "demo_role_switched" : "demo_entered";
}

// The link of the entry page in this tab. The fragment leaves the address
// bar, so a reload of the waiting room or an entry page finds it here.
const DEMO_LINK_KEY = "moto-demo-link";

function storeDemoLink(link: DemoLink): void {
  try {
    globalThis.sessionStorage.setItem(DEMO_LINK_KEY, JSON.stringify(link));
  } catch {
    // Without storage a reload shows the problem of a missing link.
  }
}

function readStoredDemoLink(): DemoLink | null {
  try {
    const raw = globalThis.sessionStorage.getItem(DEMO_LINK_KEY);
    if (!raw) return null;
    const stored = JSON.parse(raw) as Partial<DemoLink>;
    if (typeof stored.token !== "string" || !stored.token) return null;
    const link: DemoLink = isDemoRole(stored.role)
      ? { token: stored.token, role: stored.role }
      : { token: stored.token };
    if (stored.switched === true) link.switched = true;
    if (stored.restarted === true) link.restarted = true;
    return link;
  } catch {
    return null;
  }
}

/**
 * Forgets the stored link once the visitor is signed in. A restart and a
 * role switch through another app bring a new fragment, which replaces the
 * stored link anyway. Only a way back to an entry page without a fragment
 * could find it later and sign in again, perhaps in a role the visitor has
 * left in the banner since.
 */
function forgetDemoLink(): void {
  try {
    globalThis.sessionStorage.removeItem(DEMO_LINK_KEY);
  } catch {
    // Without storage there is nothing to forget.
  }
}

/**
 * Reads `#token=…&role=…`, removes the fragment from the address bar and
 * keeps the link for a reload in this tab. Without a token in the fragment it
 * returns the kept link. An unknown role counts as none: the entry page then
 * asks for one.
 */
export function takeDemoLinkFromFragment(): DemoLink | null {
  const fragment = new URLSearchParams(globalThis.location.hash.slice(1));
  if (!fragment.has("token")) return readStoredDemoLink();
  globalThis.history.replaceState(
    null,
    "",
    globalThis.location.pathname + globalThis.location.search,
  );
  const token = fragment.get("token")?.trim();
  if (!token) return null;
  const role = fragment.get("role");
  const link: DemoLink = isDemoRole(role) ? { token, role } : { token };
  if (fragment.get("switched") === "1") link.switched = true;
  if (fragment.get("restarted") === "1") link.restarted = true;
  storeDemoLink(link);
  return link;
}

/** The fragment that hands a demo link on to the demo school's entry page. */
export function demoLinkFragment(link: DemoLink): string {
  const fragment = new URLSearchParams({ token: link.token });
  if (link.role) fragment.set("role", link.role);
  if (link.switched) fragment.set("switched", "1");
  if (link.restarted) fragment.set("restarted", "1");
  return `#${fragment.toString()}`;
}

// „Demo neu anfangen" (#3470): the question before the visitor loses what
// they changed, and what happens when the restart does not work.
export const DEMO_RESTART_LABEL = "Demo neu anfangen";
export const DEMO_RESTART_TITLE = "Demo neu anfangen?";
export const DEMO_RESTART_TEXT =
  "Sie bekommen eine neue Demo-Schule mit frischen Daten. Alles, was Sie bisher geändert haben, geht verloren. Der Link aus Ihrer E-Mail gilt weiter.";
export const DEMO_RESTART_CONFIRM = "Neu anfangen";
export const DEMO_RESTART_RUNNING = "Wird vorbereitet …";

/**
 * Starts the demo over in the current role (#3470). The route reads the
 * token from its cookie, asks the backend for a fresh demo school and
 * answers with the waiting room's address, where the setup screen of the
 * first entry shows again. Null when the link behind the demo has expired.
 */
export async function restartDemo(
  role: DemoRole | undefined,
): Promise<string | null> {
  const response = await fetch("/api/demo/access/reset", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ role }),
  });
  if (response.status === 404 || response.status === 410) return null;
  if (!response.ok) throw new Error(`demo restart failed: ${response.status}`);
  const body = (await response.json()) as { entry_url?: string };
  if (!body.entry_url?.startsWith("http")) {
    throw new Error("demo restart without a waiting room");
  }
  return body.entry_url;
}

/**
 * The entry page of the parents app with the link (#3468): the same token
 * signs the visitor in there as the school's parent.
 */
export function parentsDemoEntryUrl(link: DemoLink): string {
  return parentsPortalUrl(
    `/demo${demoLinkFragment({ ...link, role: "parent" })}`,
  );
}

/**
 * Where the banner switches to a role of the other app (#3468). The route
 * reads the token from its cookie and answers with a redirect to the other
 * app's entry page, so the token never reaches a script of this page.
 */
export function demoHandoffPath(role: DemoRole): string {
  return `/api/demo/access/handoff?role=${encodeURIComponent(role)}`;
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
    return {
      status: "ready",
      schoolUrl: body.school_url,
      schoolName: body.school_name,
    };
  }
  return {
    status: body.status === "failed" ? "failed" : "preparing",
    schoolName: body.school_name,
  };
}

const POLL_INTERVAL_MS = 2000;
// The page waits as long as the backend reports "preparing": with several
// seeds at once a school can take a few minutes. Five minutes only guard
// against a seed that hangs without ever reporting "failed".
const MAX_WAIT_MS = 5 * 60_000;

export type DemoWaitOutcome =
  | { phase: "ready"; schoolUrl?: string; schoolName?: string }
  | { phase: "invalid" | "failed" | "unavailable" | "cancelled" };

/** What the setup screen shows while the school is prepared (#3464). */
export interface DemoSetupProgress {
  schoolName?: string;
  /** Index into DEMO_SETUP_STEPS of the line that runs now. */
  step: number;
}

/**
 * Polls until the demo school can be entered (#3463). `failed` is a wait that
 * ran past the safety limit; `unavailable` is a school the backend reports as
 * failed, which no further wait can fix.
 */
export async function waitForDemoSchool(
  token: string,
  run: { cancelled: boolean },
  onPreparing: (progress: DemoSetupProgress) => void,
): Promise<DemoWaitOutcome> {
  const startedAt = Date.now();
  while (!run.cancelled) {
    const progress = await fetchDemoAccessProgress(token);
    if (run.cancelled) break;
    if (progress.status === "invalid") return { phase: "invalid" };
    if (progress.status === "failed") return { phase: "unavailable" };
    if (progress.status === "ready") {
      return {
        phase: "ready",
        schoolUrl: progress.schoolUrl,
        schoolName: progress.schoolName,
      };
    }
    const waited = Date.now() - startedAt;
    if (waited >= MAX_WAIT_MS) return { phase: "failed" };
    onPreparing({
      schoolName: progress.schoolName,
      step: demoSetupStep(waited),
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
    demo?: {
      access_id?: string;
      role?: string;
      src?: string;
      fixed_role?: boolean;
    };
  };
  return {
    access_token: payload.access_token,
    refresh_token: payload.refresh_token,
    visit: {
      accessId: payload.demo?.access_id ?? "",
      role: isDemoRole(payload.demo?.role) ? payload.demo.role : body.role,
      src: payload.demo?.src,
      fixedRole: payload.demo?.fixed_role === true ? true : undefined,
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
  /** The standing demo school: its shared role cannot be switched. */
  fixedRole?: boolean;
  /**
   * The OGS name for the banner in the parents app, which knows no school
   * (#3468). It never goes to the analytics.
   */
  schoolName?: string;
  pending?: DemoEntryEvent;
}

/** What an entry reports: a first entry, a role switch or a restart. */
export type DemoEntryEvent =
  "demo_entered" | "demo_role_switched" | "demo_restarted";

function isDemoEntryEvent(value: unknown): value is DemoEntryEvent {
  return (
    value === "demo_entered" ||
    value === "demo_role_switched" ||
    value === "demo_restarted"
  );
}

/**
 * Signs in with the session's token pair and notes the visit for the banner,
 * which reports `pending` once the next page has loaded. False when the
 * sign-in failed. The parents app signs in with its own provider (#3468).
 */
export async function startDemoSession(
  session: DemoSession,
  pending: NonNullable<DemoVisit["pending"]>,
  provider: "credentials" | "parent-credentials" = "credentials",
): Promise<boolean> {
  const result = await signIn(provider, {
    redirect: false,
    internalRefresh: true,
    token: session.access_token,
    refreshToken: session.refresh_token,
  });
  if (result?.error) return false;
  saveDemoVisit({ ...session.visit, pending });
  forgetDemoLink();
  return true;
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
      fixedRole: visit.fixedRole === true ? true : undefined,
      schoolName:
        typeof visit.schoolName === "string" ? visit.schoolName : undefined,
      pending: isDemoEntryEvent(visit.pending) ? visit.pending : undefined,
    };
  } catch {
    return null;
  }
}
