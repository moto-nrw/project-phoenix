import { createLogger } from "./logger";
import { sessionFetch } from "./session-cache";

const logger = createLogger({ component: "SchoolSetupApi" });

/** Einrichtungs-Assistent für neue Schulen (#2832). */
export type SchoolSetupStepKey =
  "basics" | "team" | "rooms" | "groups" | "students" | "guardians";

export interface SchoolSetupStep {
  key: SchoolSetupStepKey;
  /** Gilt der Schritt für diese Schule? */
  applies: boolean;
  done: boolean;
  skipped: boolean;
}

export interface SchoolSetupBasics {
  presenceMode: "detailed" | "binary";
  groupMode: "fixed_groups" | "open_care";
  timetableEnabled: boolean;
  /** null, solange die Schule die Frage nicht beantwortet hat. */
  parentAppUsed: boolean | null;
}

export interface SchoolSetupState {
  /** Die Schule hat die Einrichtung abgeschlossen. */
  completed: boolean;
  /** Diese Person hat den Assistenten ausgeblendet. */
  dismissed: boolean;
  basics: SchoolSetupBasics;
  steps: SchoolSetupStep[];
}

export function schoolSetupSWRKey(
  tenantSlug: string,
  accountID: string,
): string {
  return `school-setup:${tenantSlug}:${accountID}`;
}

const STEP_KEYS: readonly SchoolSetupStepKey[] = [
  "basics",
  "team",
  "rooms",
  "groups",
  "students",
  "guardians",
];

interface BackendStep {
  key?: unknown;
  applies?: unknown;
  done?: unknown;
  skipped?: unknown;
}

interface BackendState {
  completed?: unknown;
  dismissed?: unknown;
  basics?: {
    presence_mode?: unknown;
    group_mode?: unknown;
    timetable_enabled?: unknown;
    parent_app_used?: unknown;
  };
  steps?: unknown;
}

export function mapSchoolSetupState(raw: BackendState): SchoolSetupState {
  const steps = Array.isArray(raw.steps) ? (raw.steps as BackendStep[]) : [];
  const parentApp = raw.basics?.parent_app_used;
  return {
    completed: raw.completed === true,
    dismissed: raw.dismissed === true,
    basics: {
      presenceMode:
        raw.basics?.presence_mode === "binary" ? "binary" : "detailed",
      groupMode:
        raw.basics?.group_mode === "open_care" ? "open_care" : "fixed_groups",
      timetableEnabled: raw.basics?.timetable_enabled === true,
      parentAppUsed: typeof parentApp === "boolean" ? parentApp : null,
    },
    steps: steps
      .filter((step): step is BackendStep & { key: SchoolSetupStepKey } =>
        STEP_KEYS.includes(step.key as SchoolSetupStepKey),
      )
      .map((step) => ({
        key: step.key,
        applies: step.applies === true,
        done: step.done === true,
        skipped: step.skipped === true,
      })),
  };
}

async function readState(response: Response, event: string) {
  if (!response.ok) {
    logger.error(event, { status: response.status });
    throw new SchoolSetupError(response.status);
  }
  const body = (await response.json()) as
    { data?: BackendState } | BackendState;
  const data = "data" in body && body.data ? body.data : (body as BackendState);
  // Eine offene Einrichtung hat immer Schritte. Fehlen sie, ist die Antwort
  // nicht die erwartete; sie darf den Stand nicht ersetzen, sonst sähe die
  // Checkliste „alles erledigt“ (0 von 0).
  if (data.completed !== true && !Array.isArray(data.steps)) {
    logger.error(event, { reason: "unexpected_shape" });
    throw new SchoolSetupError(response.status);
  }
  return mapSchoolSetupState(data);
}

/** Fehler mit dem HTTP-Status, damit der Assistent passend antworten kann. */
export class SchoolSetupError extends Error {
  constructor(readonly status: number) {
    super(`school setup request failed (${status})`);
  }
}

/**
 * Liest den Assistenten für die angemeldete Person. Ohne Recht (401/403)
 * gibt es keinen Assistenten: dann `null`.
 */
export async function fetchSchoolSetup(): Promise<SchoolSetupState | null> {
  const response = await sessionFetch("/api/school-setup", { method: "GET" });
  if (response.status === 401 || response.status === 403) {
    return null;
  }
  return readState(response, "fetch_school_setup_failed");
}

async function send(
  path: string,
  method: "PUT" | "POST",
  body: unknown,
  event: string,
): Promise<SchoolSetupState> {
  const response = await sessionFetch(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  return readState(response, event);
}

export function confirmSchoolSetupBasics(
  presenceMode: SchoolSetupBasics["presenceMode"],
  parentAppUsed: boolean,
): Promise<SchoolSetupState> {
  return send(
    "/api/school-setup/basics",
    "PUT",
    { presence_mode: presenceMode, parent_app_used: parentAppUsed },
    "confirm_school_setup_basics_failed",
  );
}

export function setSchoolSetupStepSkipped(
  step: SchoolSetupStepKey,
  skipped: boolean,
): Promise<SchoolSetupState> {
  return send(
    `/api/school-setup/steps/${step}`,
    "PUT",
    { skipped },
    "skip_school_setup_step_failed",
  );
}

export function completeSchoolSetup(): Promise<SchoolSetupState> {
  return send(
    "/api/school-setup/complete",
    "POST",
    undefined,
    "complete_school_setup_failed",
  );
}

export function setSchoolSetupDismissed(
  dismissed: boolean,
): Promise<SchoolSetupState> {
  return send(
    "/api/school-setup/dismissal",
    "PUT",
    { dismissed },
    "dismiss_school_setup_failed",
  );
}
