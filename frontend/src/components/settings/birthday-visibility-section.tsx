"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { Cake } from "lucide-react";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Skeleton } from "~/components/ui/skeleton";
import { SectionCard } from "~/components/ui/section-card";
import { BooleanField } from "~/components/settings/fields/boolean-field";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { ApiError, wireErrorCode } from "~/lib/api-error";
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
 * answers workforce.staff_profile_missing) rather than offering a setting that cannot apply to anyone.
 */
export function BirthdayVisibilitySection() {
  const [visible, setVisible] = useState(true);
  const [loading, setLoading] = useState(true);
  const [available, setAvailable] = useState(true);
  const [busy, setBusy] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  // Laden: Fehler vor Ort mit Wiederholen. Umschalten ist ein Schalter ohne
  // Formular: ein Fehler kommt als Toast (#2517).
  const { error: loadError, show: showLoadError, clear } = useApiLoadError();
  const { show: showSaveError } = useApiErrorDisplay();
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestToggleRef = useRef<(next: boolean) => void>(() => undefined);

  const load = useCallback(
    async (isCancelled: () => boolean = () => false) => {
      setLoading(true);
      setLoadFailed(false);
      clear();
      try {
        const optOut = await fetchBirthdayOptOut();
        if (!isCancelled()) setVisible(!optOut);
      } catch (err) {
        if (isCancelled()) return;
        // Konten ohne Personaldatensatz haben nichts abzuwählen: die Karte
        // blendet sich aus, statt einen Fehler zu zeigen.
        if (
          err instanceof ApiError &&
          wireErrorCode(err.code) === "workforce.staff_profile_missing"
        ) {
          setAvailable(false);
          return;
        }
        setLoadFailed(true);
        logger.error("birthday_opt_out_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showLoadError(err, {
          object: "die Einstellung zum Geburtstag",
          retry: () => latestLoadRef.current(),
        });
      } finally {
        if (!isCancelled()) setLoading(false);
      }
    },
    [clear, showLoadError],
  );

  useEffect(() => {
    let cancelled = false;
    void load(() => cancelled);
    return () => {
      cancelled = true;
    };
  }, [load]);

  const toggle = async (nextVisible: boolean) => {
    // Optimistic: the switch answers immediately and rolls back on failure.
    setBusy(true);
    setVisible(nextVisible);
    try {
      await updateBirthdayOptOut(!nextVisible);
    } catch (err) {
      logger.error("birthday_opt_out_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setVisible(!nextVisible);
      void showSaveError(err, {
        object: "die Einstellung zum Geburtstag",
        retry: () => latestToggleRef.current(nextVisible),
      });
    } finally {
      setBusy(false);
    }
  };

  useLayoutEffect(() => {
    latestLoadRef.current = () => void load();
    latestToggleRef.current = (next) => void toggle(next);
  });

  if (!available) return null;

  return (
    <SectionCard
      icon={Cake}
      headingLevel={3}
      title="Geburtstag"
      description="Ihr Name erscheint in der Geburtstagsübersicht auf der Startseite, ohne Geburtsjahr."
    >
      {/* Bis der Text eines Ladefehlers da ist, bleibt das Skelett stehen:
          kein Schalter mit geratenem Stand. */}
      {loading || (loadFailed && !loadError) ? (
        <Skeleton className="h-10 w-full" />
      ) : loadFailed ? (
        <LoadErrorAlert error={loadError} />
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
    </SectionCard>
  );
}
