import { createLogger } from "./logger";
import { sessionFetch } from "./session-cache";

const logger = createLogger({ component: "StaffOnboardingApi" });

/** Die ersten Schritte einer Betreuungskraft (#3748). */
export type StaffOnboardingStepKey =
  | "groups"
  | "students"
  | "attendance"
  | "calendar"
  | "supervision"
  | "work_time";

export const STAFF_ONBOARDING_STEP_KEYS: readonly StaffOnboardingStepKey[] = [
  "groups",
  "students",
  "attendance",
  "calendar",
  "supervision",
  "work_time",
];

export type StaffOnboardingStepState = "open" | "done" | "skipped";

export interface StaffOnboardingState {
  /** Diese Person hat die Checkliste ausgeblendet. */
  dismissed: boolean;
  /** Die Schule hat eine Gruppe oder ein Kind; vorher zeigten die Touren leere Seiten. */
  schoolReady: boolean;
  doneSteps: readonly StaffOnboardingStepKey[];
  skippedSteps: readonly StaffOnboardingStepKey[];
}

export function staffOnboardingSWRKey(
  tenantSlug: string,
  accountID: string,
): string {
  return `staff-onboarding:${tenantSlug}:${accountID}`;
}

interface BackendState {
  dismissed?: unknown;
  school_ready?: unknown;
  done_steps?: unknown;
  skipped_steps?: unknown;
}

function stepKeys(raw: unknown): StaffOnboardingStepKey[] {
  if (!Array.isArray(raw)) return [];
  return raw.filter((key): key is StaffOnboardingStepKey =>
    STAFF_ONBOARDING_STEP_KEYS.includes(key as StaffOnboardingStepKey),
  );
}

export function mapStaffOnboardingState(
  raw: BackendState,
): StaffOnboardingState {
  return {
    dismissed: raw.dismissed === true,
    schoolReady: raw.school_ready === true,
    doneSteps: stepKeys(raw.done_steps),
    skippedSteps: stepKeys(raw.skipped_steps),
  };
}

/** Fehler mit dem HTTP-Status, damit die Checkliste passend antworten kann. */
export class StaffOnboardingError extends Error {
  constructor(readonly status: number) {
    super(`staff onboarding request failed (${status})`);
  }
}

async function readState(
  response: Response,
  event: string,
): Promise<StaffOnboardingState> {
  if (!response.ok) {
    logger.error(event, { status: response.status });
    throw new StaffOnboardingError(response.status);
  }
  const body = (await response.json()) as
    { data?: BackendState } | BackendState;
  const data = "data" in body && body.data ? body.data : (body as BackendState);
  // Ohne die beiden Listen ist die Antwort nicht die erwartete. Sie darf den
  // Stand nicht ersetzen, sonst stünde jeder Schritt wieder offen.
  if (!Array.isArray(data.done_steps) || !Array.isArray(data.skipped_steps)) {
    logger.error(event, { reason: "unexpected_shape" });
    throw new StaffOnboardingError(response.status);
  }
  return mapStaffOnboardingState(data);
}

/**
 * Liest die ersten Schritte der angemeldeten Person. Ohne Konto in der Schule
 * (401/403) gibt es keine Checkliste: dann `null`.
 */
export async function fetchStaffOnboarding(): Promise<StaffOnboardingState | null> {
  const response = await sessionFetch("/api/staff-onboarding", {
    method: "GET",
  });
  if (response.status === 401 || response.status === 403) {
    return null;
  }
  return readState(response, "fetch_staff_onboarding_failed");
}

async function put(
  path: string,
  body: unknown,
  event: string,
): Promise<StaffOnboardingState> {
  const response = await sessionFetch(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return readState(response, event);
}

export function setStaffOnboardingStepState(
  step: StaffOnboardingStepKey,
  state: StaffOnboardingStepState,
): Promise<StaffOnboardingState> {
  return put(
    `/api/staff-onboarding/steps/${step}`,
    { state },
    "set_staff_onboarding_step_failed",
  );
}

export function setStaffOnboardingDismissed(
  dismissed: boolean,
): Promise<StaffOnboardingState> {
  return put(
    "/api/staff-onboarding/dismissal",
    { dismissed },
    "dismiss_staff_onboarding_failed",
  );
}
