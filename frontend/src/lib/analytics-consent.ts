/**
 * The signed-in person's consent to the usage analytics (§ 25 TDDDG, #4901).
 *
 * Even without cookies the PostHog SDK accesses the device, so it needs the
 * person's own consent; the school's switch cannot replace it. The answer
 * belongs to the account (`GET`/`PUT /analytics-consent/` in the backend),
 * one answer for every portal and school. This module holds the answer of the
 * current portal session, hands it to the analytics context, and serves the
 * prompt after login and the setting in the profile. Unknown, refused, or
 * failed reads keep collection off; a revocation applies before it is saved.
 */

import { useSyncExternalStore } from "react";
// zod/mini: the full "zod" build adds ~20 kB to every route's client bundle.
import * as z from "zod/mini";
import { clientEnv } from "~/env.client";
import { analyticsDeployment } from "~/lib/analytics-deployment";
import type { AnalyticsSurface } from "~/lib/analytics-policy";
import { apiErrorFromBody, transportFetch } from "~/lib/api-error";
import {
  analyticsHostSurface,
  setAnalyticsContext,
} from "~/lib/posthog-client";

export type AnalyticsConsentPortal = "tenant" | "parent" | "school";

const PORTAL_SURFACES: Readonly<
  Record<AnalyticsConsentPortal, AnalyticsSurface>
> = {
  tenant: "ogs",
  parent: "parents",
  school: "school",
};

const PORTAL_PATHS: Readonly<Record<AnalyticsConsentPortal, string>> = {
  tenant: "/api/analytics-consent",
  parent: "/api/parent/analytics-consent",
  school: "/api/school/analytics-consent",
};

export interface AnalyticsConsentState {
  /** The portal whose session answers; null without a signed-in session. */
  readonly portal: AnalyticsConsentPortal | null;
  readonly status: "idle" | "loading" | "error" | "ready";
  /** The stored answer; null while the person has not decided. */
  readonly granted: boolean | null;
  /** Why the answer could not be read, for the shared error display. */
  readonly error: unknown;
  /** The school lets the analytics run; asking is pointless otherwise. */
  readonly schoolAllows: boolean;
  /** The prompt was closed without an answer, for this page only. */
  readonly dismissed: boolean;
}

const IDLE: AnalyticsConsentState = {
  portal: null,
  status: "idle",
  granted: null,
  error: null,
  schoolAllows: false,
  dismissed: false,
};

const consentResponseSchema = z.object({
  data: z.object({ granted: z.nullable(z.boolean()) }),
});

let state = IDLE;
let sessionKey: string | null = null;
let sequence = 0;
const listeners = new Set<() => void>();

function update(next: Partial<AnalyticsConsentState>): void {
  state = { ...state, ...next };
  for (const listener of listeners) listener();
}

function ownsHost(portal: AnalyticsConsentPortal): boolean {
  return (
    typeof window !== "undefined" &&
    analyticsHostSurface() === PORTAL_SURFACES[portal]
  );
}

function publishConsent(consent: boolean): void {
  setAnalyticsContext({ consent });
}

async function requestConsent(
  portal: AnalyticsConsentPortal,
  init?: RequestInit,
): Promise<boolean | null> {
  const response = await transportFetch(PORTAL_PATHS[portal], {
    cache: "no-store",
    credentials: "include",
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  const text = await response.text();
  let body: unknown = null;
  try {
    body = text ? (JSON.parse(text) as unknown) : null;
  } catch {
    body = null;
  }
  if (!response.ok) {
    throw apiErrorFromBody(
      `Nutzungsstatistik-Einwilligung fehlgeschlagen (${response.status}).`,
      response.status,
      body,
    );
  }
  return consentResponseSchema.parse(body).data.granted;
}

/**
 * Whether consent is asked for at all: a school deployment whose analytics
 * is configured. The public demo has its own rules and no accounts to ask.
 */
export function analyticsConsentApplies(): boolean {
  return (
    Boolean(clientEnv.NEXT_PUBLIC_POSTHOG_KEY) &&
    analyticsDeployment() !== "demo"
  );
}

/**
 * Reads the answer of a signed-in portal session. Only the portal that owns
 * this host may start it; the same session is read once, a new account starts
 * over with collection off until its answer arrives.
 */
export function startAnalyticsConsent(
  portal: AnalyticsConsentPortal,
  accountKey: string,
): void {
  if (!ownsHost(portal)) return;
  const key = `${portal}:${accountKey}`;
  if (sessionKey === key) {
    // A cleared analytics context forgets the answer; the session keeps it.
    if (state.status === "ready") publishConsent(state.granted === true);
    return;
  }
  sessionKey = key;
  publishConsent(false);
  update({ ...IDLE, portal, status: "loading" });
  void loadAnalyticsConsent();
}

/** (Re)reads the current session's answer; also the retry of a failed read. */
export async function loadAnalyticsConsent(): Promise<void> {
  const portal = state.portal;
  if (!portal) return;
  const current = ++sequence;
  update({ status: "loading", error: null });
  try {
    const granted = await requestConsent(portal);
    if (current !== sequence) return;
    publishConsent(granted === true);
    update({ status: "ready", granted });
  } catch (error) {
    if (current !== sequence) return;
    publishConsent(false);
    update({ status: "error", error });
  }
}

/** Logout or a session of another account: back to no answer, no collection. */
export function stopAnalyticsConsent(portal: AnalyticsConsentPortal): void {
  if (state.portal !== portal && !ownsHost(portal)) return;
  sessionKey = null;
  sequence++;
  if (state.portal) publishConsent(false);
  update(IDLE);
}

/** The school's own switch, so nobody is asked while the school is off. */
export function setAnalyticsConsentSchool(
  portal: AnalyticsConsentPortal,
  allows: boolean,
): void {
  if (state.portal !== portal || state.schoolAllows === allows) return;
  update({ schoolAllows: allows });
}

/**
 * Records the person's answer. A refusal or revocation stops collection at
 * once, before the request; a consent applies only once it is stored.
 * Throws the request's ApiError for the caller's error display.
 */
export async function decideAnalyticsConsent(granted: boolean): Promise<void> {
  const portal = state.portal;
  if (!portal) return;
  if (!granted) publishConsent(false);
  const current = ++sequence;
  const stored = await requestConsent(portal, {
    method: "PUT",
    body: JSON.stringify({ granted }),
  });
  if (current !== sequence) return;
  publishConsent(stored === true);
  update({ status: "ready", granted: stored, error: null, dismissed: true });
}

/** Closes the prompt without an answer; it returns with the next page load. */
export function dismissAnalyticsConsentPrompt(): void {
  update({ dismissed: true });
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useAnalyticsConsent(): AnalyticsConsentState {
  return useSyncExternalStore(
    subscribe,
    () => state,
    () => IDLE,
  );
}
