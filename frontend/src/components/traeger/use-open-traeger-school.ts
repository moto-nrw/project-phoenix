"use client";

import { useSession } from "next-auth/react";
import { useCallback, useState } from "react";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { endTraegerSession, useShellAuthSafe } from "~/lib/shell-auth-context";
import { requestTraegerSignInWithCode } from "~/lib/traeger-sign-in-with-code";
import {
  startTraegerCodeStep,
  startTraegerSchoolSession,
  type TraegerCodeStep,
  type TraegerSchool,
} from "~/lib/traeger-api";
import { traegerHandoffUrl } from "~/lib/traeger-handoff";
import { TRAEGER_ROLE_LABELS } from "~/lib/traeger-roles";

const logger = createLogger({ component: "useOpenTraegerSchool" });

/** The Träger right „In OGS wechseln" (ADR 0084, #4654). */
const SCHOOL_SWITCH_RIGHT = "traeger_school:switch";

/**
 * Without the right the button is missing; this names the role that carries
 * it and the role that hands it out (#5473, #5471).
 */
export const SCHOOL_SWITCH_MISSING_HINT = `Die Schul-App öffnen Sie nur mit der Rolle „${TRAEGER_ROLE_LABELS.school_access}“. Diese Rolle gibt Ihnen eine Person mit der Rolle „${TRAEGER_ROLE_LABELS.leadership}“.`;

/**
 * Opens the school in a new tab: the tab opens on the click, so the browser
 * does not block it, and receives the school's address once the hand-off
 * token is there. Without a tab the school opens here.
 */
type OpenableSchool = Pick<TraegerSchool, "id" | "subdomain">;

async function openSchool(school: OpenableSchool): Promise<void> {
  const tab = globalThis.open("", "_blank");
  try {
    const url = traegerHandoffUrl(
      school.subdomain,
      await startTraegerSchoolSession(school.id),
    );
    if (tab) {
      tab.opener = null;
      tab.location.href = url;
    } else {
      globalThis.location.assign(url);
    }
  } catch (err) {
    tab?.close();
    throw err;
  }
}

/** Label of the action that mails the code for the running session. */
const TRAEGER_CODE_STEP_LABEL = "Code anfordern";

/**
 * A school that switched remembered devices off asks for the code (#6955).
 * The person enters it in the running session (#7068) and the school opens.
 */
function needsCode(err: unknown): boolean {
  return err instanceof ApiError && err.code === "traeger.school_requires_code";
}

/** The action „Schul-App öffnen" of the school list and the school view. */
export function useOpenTraegerSchool() {
  const { data: session } = useSession();
  const [opening, setOpening] = useState<string | null>(null);
  const [codeStep, setCodeStep] = useState<{
    school: OpenableSchool;
    step: TraegerCodeStep;
  } | null>(null);
  const openErrors = useApiErrorDisplay();
  const shellAuth = useShellAuthSafe();

  const signInWithCode = useCallback(async () => {
    requestTraegerSignInWithCode();
    // The shell's logout reports its own failure; without the shell the
    // same sequence runs here (revoke, clear, barrier; #5194, #5000, #5399).
    if (shellAuth) {
      await shellAuth.logout();
      return;
    }
    try {
      await endTraegerSession();
    } catch (err) {
      // A failed revocation keeps the session; the person may try again.
      void openErrors.show(err, { object: "die Abmeldung" });
      throw err;
    }
  }, [shellAuth, openErrors]);

  const requestCode = useCallback(
    async (school: OpenableSchool) => {
      try {
        setCodeStep({ school, step: await startTraegerCodeStep() });
      } catch (err) {
        logger.warn("traeger_code_step_start_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void openErrors.show(err, { object: "das Senden des Codes" });
      }
    },
    [openErrors],
  );

  const open = useCallback(
    async (school: OpenableSchool) => {
      setOpening(school.id);
      try {
        await openSchool(school);
      } catch (err) {
        const askForCode = needsCode(err);
        // The school's „Aus“ for remembered devices is an expected answer,
        // not a failure of the app.
        if (askForCode) {
          logger.info("traeger_school_requires_code");
        } else {
          logger.error("traeger_school_open_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
        void openErrors.show(err, {
          object: "die Schule",
          ...(askForCode && {
            action: {
              label: TRAEGER_CODE_STEP_LABEL,
              onClick: () => void requestCode(school),
            },
          }),
        });
      } finally {
        setOpening(null);
      }
    },
    [openErrors, requestCode],
  );

  const closeCodeStep = useCallback(() => setCodeStep(null), []);

  const codeVerified = useCallback(
    async (renewed: boolean) => {
      const school = codeStep?.school;
      setCodeStep(null);
      if (!renewed) {
        // The code was right, but the renewed session could not be kept:
        // the old one is retired, so the person signs in again.
        await signInWithCode().catch((signOutError: unknown) => {
          logger.warn("traeger_sign_in_with_code_failed", {
            error:
              signOutError instanceof Error
                ? signOutError.message
                : String(signOutError),
          });
        });
        return;
      }
      if (school) await open(school);
    },
    [codeStep, open, signInWithCode],
  );

  return {
    canOpen: hasPermission(session, SCHOOL_SWITCH_RIGHT),
    opening,
    open,
    /** Props of the TraegerCodeStepModal the page renders. */
    codeStep: {
      step: codeStep?.step ?? null,
      onClose: closeCodeStep,
      onVerified: codeVerified,
    },
  };
}
