"use client";

import { useSession } from "next-auth/react";
import { useCallback, useState } from "react";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { createLogger } from "~/lib/logger";
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

/** The action „Schul-App öffnen" of the school list and the school view. */
export function useOpenTraegerSchool() {
  const { data: session } = useSession();
  const [opening, setOpening] = useState<string | null>(null);
  const openErrors = useApiErrorDisplay();

  const open = useCallback(
    async (school: Pick<TraegerSchool, "id" | "subdomain">) => {
      setOpening(school.id);
      try {
        await openSchool(school);
      } catch (err) {
        logger.error("traeger_school_open_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void openErrors.show(err, { object: "die Schule" });
      } finally {
        setOpening(null);
      }
    },
    [openErrors],
  );

  return {
    canOpen: hasPermission(session, SCHOOL_SWITCH_RIGHT),
    opening,
    open,
  };
}
