"use client";

import { signOut, useSession } from "next-auth/react";
import { useCallback, useState } from "react";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
import { portalLoginPath } from "~/lib/portal-paths";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import { requestTraegerSignInWithCode } from "~/lib/traeger-sign-in-with-code";
import {
  startTraegerSchoolSession,
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
async function openSchool(
  school: Pick<TraegerSchool, "id" | "subdomain">,
): Promise<void> {
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

/**
 * A school that switched remembered devices off asks for the code (#6955):
 * the person signs out and in again, this time with the code.
 */
function needsSignInWithCode(err: unknown): boolean {
  return err instanceof ApiError && err.code === "traeger.school_requires_code";
}

/** The action „Schul-App öffnen" of the school list and the school view. */
export function useOpenTraegerSchool() {
  const { data: session } = useSession();
  const [opening, setOpening] = useState<string | null>(null);
  const openErrors = useApiErrorDisplay();
  const shellAuth = useShellAuthSafe();

  const signInWithCode = useCallback(async () => {
    requestTraegerSignInWithCode();
    if (shellAuth) {
      await shellAuth.logout();
      return;
    }
    await signOut({ redirect: false });
    globalThis.location.assign(portalLoginPath("traeger"));
  }, [shellAuth]);

  const open = useCallback(
    async (school: Pick<TraegerSchool, "id" | "subdomain">) => {
      setOpening(school.id);
      try {
        await openSchool(school);
      } catch (err) {
        const askForCode = needsSignInWithCode(err);
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
              label: "Mit Code neu anmelden",
              // A failed sign-out leaves the person signed in; opening the
              // school again offers the action again.
              onClick: () =>
                void signInWithCode().catch((signOutError: unknown) => {
                  logger.warn("traeger_sign_in_with_code_failed", {
                    error:
                      signOutError instanceof Error
                        ? signOutError.message
                        : String(signOutError),
                  });
                }),
            },
          }),
        });
      } finally {
        setOpening(null);
      }
    },
    [openErrors, signInWithCode],
  );

  return {
    canOpen: hasPermission(session, SCHOOL_SWITCH_RIGHT),
    opening,
    open,
  };
}
