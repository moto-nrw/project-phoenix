"use client";

import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { createLogger } from "~/lib/logger";
import { fetchTraegerOrganization } from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerTestSystemNotice" });

/**
 * Marks a test Träger (#4658) on every page of the Träger portal, so nobody
 * mistakes the invented sample data for real children or enters real data.
 * A real Träger shows nothing. A failed read shows nothing either: the pages
 * report their own load errors, and the session guard handles a session the
 * backend ended.
 */
export function TraegerTestSystemNotice() {
  const [testSystem, setTestSystem] = useState(false);

  useEffect(() => {
    let active = true;
    fetchTraegerOrganization()
      .then((organization) => {
        if (active) setTestSystem(organization.testSystem);
      })
      .catch((err: unknown) => {
        // Silent on purpose: the notice is an extra on top of every page,
        // and each page shows its own load error.
        logger.warn("traeger_organization_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      active = false;
    };
  }, []);

  if (!testSystem) return null;
  return (
    <Alert
      type="info"
      title="Testsystem"
      announce="off"
      message="Hier probieren Sie moto aus. Alle Kinder sind erfunden. Tragen Sie hier keine echten Daten ein."
      className="mb-6"
    />
  );
}
