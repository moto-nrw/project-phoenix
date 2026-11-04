"use client";

import { useEffect, useState } from "react";
import { createLogger } from "~/lib/logger";
import {
  fetchTraegerOrganization,
  type TraegerOrganization,
} from "~/lib/traeger-api";

const logger = createLogger({ component: "useTraegerOrganization" });

/**
 * The Träger of the session, for the shell (#5145): its name next to the
 * logo and the test-system strip (#4658). `null` while it loads and after a
 * failed read: the shell then shows the logo alone and no strip, the pages
 * report their own load errors, and the session guard handles a session the
 * backend ended.
 */
export function useTraegerOrganization(): TraegerOrganization | null {
  const [organization, setOrganization] = useState<TraegerOrganization | null>(
    null,
  );

  useEffect(() => {
    let active = true;
    fetchTraegerOrganization()
      .then((loaded) => {
        if (active) setOrganization(loaded);
      })
      .catch((err: unknown) => {
        // Silent on purpose: name and strip are extras on top of every page.
        logger.warn("traeger_organization_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      active = false;
    };
  }, []);

  return organization;
}
