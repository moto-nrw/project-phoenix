"use client";

import Link from "next/link";
import { useState } from "react";

import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { InfoCard } from "~/components/ui/info-card";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";
import {
  acknowledgeStaffNotice,
  describeRecurrence,
  fetchTodaysNotices,
} from "~/lib/staff-notices-api";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

// Wie viele Hinweise auf der Startseite ausgeschrieben stehen. Mehr als zwei
// schieben alles andere aus dem Bild; der Rest steht eine Seite weiter.
const PREVIEW_LIMIT = 2;

const logger = createLogger({ component: "TagesinfoCard" });

/**
 * "Tagesinformationen" (#2180): die Hinweise der Leitung, die HEUTE gelten.
 *
 * Steht direkt unter dem Tagesstatus, weil ein Hinweis für den ganzen Tag gilt
 * und niemand danach sucht — er muss einem begegnen. Rendert null, wenn heute
 * nichts anliegt: eine leere Tafel wäre tägliches Rauschen.
 *
 * Wichtige Hinweise stehen zuerst (Sortierung kommt aus dem Backend) und
 * tragen ein Kennzeichen. Wo eine Kenntnisnahme verlangt ist, steht eine
 * Schaltfläche — bestätigt wird der Hinweis, nicht der Tag: ein
 * wiederkehrender Hinweis fragt nicht jeden Dienstag erneut.
 *
 * Anders als die übrigen Sektionen sind das keine Zeilen, sondern Texte: der
 * Hinweis IST der Inhalt, nicht der Verweis auf einen Inhalt. Deshalb steht
 * er ausgeschrieben da, mit Luft zwischen den Hinweisen statt Trennlinien.
 */
export function TagesinfoCard() {
  const tenantPath = useTenantAwarePath();
  const { data, mutate } = useSWRAuth<StaffNotice[]>(
    "dashboard-staff-notices",
    fetchTodaysNotices,
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const notices = data ?? [];
  if (notices.length === 0) return null;

  const confirm = async (notice: StaffNotice) => {
    setPending(notice.id);
    setError(null);
    try {
      await acknowledgeStaffNotice(notice.id);
      await mutate();
    } catch (err) {
      logger.error("staff_notice_acknowledge_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        getApiErrorMessage(
          err,
          "bestätigen",
          "die Tagesinformation",
          "Die Kenntnisnahme konnte nicht gespeichert werden.",
        ),
      );
    } finally {
      setPending(null);
    }
  };

  return (
    <InfoCard
      title="Tagesinformationen"
      icon={<MotoConceptIcon concept="announcements" size={20} />}
    >
      <p className="mb-3 text-sm text-gray-600">
        Hinweise der Leitung für heute.
      </p>
      {error && <p className="text-moto-red-strong mb-3 text-sm">{error}</p>}
      <ul className="space-y-3">
        {notices.slice(0, PREVIEW_LIMIT).map((notice) => (
          <li key={notice.id}>
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="text-[15px] font-semibold text-gray-900">
                {notice.title}
              </h3>
              {notice.priority === "important" && (
                <StatusBadge label="Wichtig" tone="orange" />
              )}
              <span className="text-xs text-gray-500">
                {describeRecurrence(notice)}
              </span>
            </div>

            {notice.body && (
              // Drei Zeilen reichen für den Kern; wer mehr braucht, öffnet den
              // Hinweis. Ein langer Text hier schiebt den Rest des Tages
              // unter die Bildkante.
              <p className="mt-1 line-clamp-2 max-w-3xl text-sm leading-6 whitespace-pre-line text-gray-600">
                {notice.body}
              </p>
            )}

            {notice.requires_acknowledgement && (
              <div className="mt-2">
                {notice.acknowledged_at ? (
                  <StatusBadge label="Zur Kenntnis genommen" tone="green" />
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    disabled={pending === notice.id}
                    onClick={() => void confirm(notice)}
                  >
                    {pending === notice.id
                      ? "Wird gespeichert …"
                      : "Zur Kenntnis genommen"}
                  </Button>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>

      {notices.length > PREVIEW_LIMIT && (
        <Link
          href={tenantPath("/tagesinformationen")}
          className="text-moto-blue-strong mt-3 inline-block text-sm font-medium hover:underline"
        >
          Alle {notices.length} Hinweise
        </Link>
      )}
    </InfoCard>
  );
}
