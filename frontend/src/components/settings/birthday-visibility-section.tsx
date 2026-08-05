"use client";

import { useEffect, useState } from "react";
import { Cake } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { createLogger } from "~/lib/logger";
import { fetchBirthdayOptOut, updateBirthdayOptOut } from "~/lib/birthdays-api";

const logger = createLogger({ component: "BirthdayVisibilitySection" });

/**
 * Personal opt-out from the birthday display (#1542).
 *
 * Self-service on purpose: the school setting decides whether staff birthdays
 * may be shown at all, but whether MY name appears on a screen every colleague
 * sees is my decision, not an admin's. The switch below is phrased positively
 * ("mein Geburtstag darf erscheinen") and stored inverted as the opt-out.
 *
 * The card hides itself for accounts without a staff record (the backend
 * answers 404) rather than offering a setting that cannot apply to anyone.
 */
export function BirthdayVisibilitySection() {
  const [visible, setVisible] = useState(true);
  const [loading, setLoading] = useState(true);
  const [available, setAvailable] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const optOut = await fetchBirthdayOptOut();
        if (!cancelled) setVisible(!optOut);
      } catch (err) {
        if (!cancelled) setAvailable(false);
        logger.info("birthday_opt_out_unavailable", {
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const toggle = async (nextVisible: boolean) => {
    // Optimistic: the switch answers immediately and rolls back on failure.
    setBusy(true);
    setVisible(nextVisible);
    setError(null);
    try {
      await updateBirthdayOptOut(!nextVisible);
    } catch (err) {
      logger.error("birthday_opt_out_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setVisible(!nextVisible);
      setError("Die Einstellung konnte nicht gespeichert werden.");
    } finally {
      setBusy(false);
    }
  };

  if (!available) return null;

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <div className="mb-4 flex items-center gap-2">
        <Cake className="h-5 w-5 text-gray-800" aria-hidden="true" />
        <h3 className="text-base font-semibold text-gray-900">Geburtstag</h3>
      </div>

      <p className="mb-4 text-sm text-gray-600">
        Wenn Ihre Schule die Geburtstagsanzeige für das Team eingeschaltet hat,
        erscheint Ihr Name an Ihrem Geburtstag auf der Startseite. Ihr
        Geburtsjahr wird dabei nie angezeigt.
      </p>

      {error && (
        <div className="mb-3">
          <Alert type="error" message={error} />
        </div>
      )}

      {loading ? (
        <Skeleton className="h-10 w-full" />
      ) : (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-gray-200/50 bg-gray-50/50 p-3">
          <span className="text-sm text-gray-800">
            Meinen Geburtstag auf der Startseite anzeigen
          </span>
          <BooleanField
            value={visible}
            onChange={(next) => void toggle(next)}
            disabled={busy}
            ariaLabel="Meinen Geburtstag auf der Startseite anzeigen"
          />
        </div>
      )}
    </div>
  );
}
