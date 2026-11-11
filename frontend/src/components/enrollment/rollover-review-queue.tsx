"use client";

import { useRefreshPhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";
import { MAX_SUPPORTED_GRADE_LEVEL } from "~/lib/grade-level";
import { useCallback, useLayoutEffect, useRef, useState } from "react";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import {
  decideRolloverReview,
  listRolloverReview,
  type ReviewQueueItem,
} from "~/lib/enrollment-phase-api";
import { ApiError } from "~/lib/api-error";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { createLogger } from "~/lib/logger";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";

const logger = createLogger({ component: "RolloverReviewQueue" });

const REVIEW_REASON_LABELS: Record<string, string> = {
  grade_above_max: "Klassenstufe über der Höchstgrenze",
  no_grade_level: "Keine Klassenstufe hinterlegt",
};

interface Props {
  readonly phaseID: string;
}

const REVIEW_QUEUE_DESCRIPTION =
  "Kinder, die nicht automatisch übernommen werden konnten, meist weil ihre Klassenstufe nach Erhöhung über der Höchstgrenze liegt. Wählen Sie pro Eintrag, ob Sie das Kind behalten (ggf. mit anderer Klassenstufe), aus der nächsten Phase entfernen oder vorerst zurückstellen.";

function gradeFieldName(childId: string): string {
  return `grade-${childId}`;
}

export function RolloverReviewQueue({ phaseID }: Props) {
  const refreshPhaseExpiryWarnings = useRefreshPhaseExpiryWarnings();
  const toast = useToast();
  const [items, setItems] = useState<ReviewQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [accessRefused, setAccessRefused] = useState(false);
  const activePhase = useRef(phaseID);
  const latestRequest = useRef(0);
  const completed = useRef(new Set<string>());
  const listLoad = useApiLoadError();
  const showLoadError = listLoad.show;
  const clearLoadError = listLoad.clear;
  const decision = useApiErrorDisplay();
  const [busyChild, setBusyChild] = useState<string | null>(null);
  const [classOverrides, setClassOverrides] = useState<Record<string, string>>(
    {},
  );
  // „Wiederholen“ lädt die Liste mit der aktuellen Fassung neu.
  const latestLoad = useRef<() => Promise<void>>(async () => undefined);

  // Rejects on failure: after a decision the caller shows it as a reload of
  // a saved change and keeps the shown list (#6652).
  const readQueue = useCallback(async () => {
    const request = ++latestRequest.current;
    try {
      const list = await listRolloverReview(phaseID);
      if (request !== latestRequest.current) return;
      setItems(list.filter((item) => !completed.current.has(item.child_id)));
      setLoadFailed(false);
      setAccessRefused(false);
      clearLoadError();
    } catch (error) {
      if (request !== latestRequest.current) return;
      setLoadFailed(true);
      if (
        error instanceof ApiError &&
        (error.status === 401 || error.status === 403)
      ) {
        setAccessRefused(true);
        setItems([]);
        setClassOverrides({});
        setBusyChild(null);
      }
      throw error;
    } finally {
      if (request === latestRequest.current) setLoading(false);
    }
  }, [phaseID, clearLoadError]);
  // Never rejects: a failed load shows its error in place of the list.
  const load = useCallback(async () => {
    const request = latestRequest.current + 1;
    setLoading(true);
    clearLoadError();
    try {
      await readQueue();
    } catch (err) {
      if (request !== latestRequest.current) return;
      logger.error("review_queue_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Prüfliste",
        retry: () => void latestLoad.current(),
      });
    } finally {
      if (request === latestRequest.current) setLoading(false);
    }
  }, [readQueue, showLoadError, clearLoadError]);
  const afterDecide = useRefreshAfterWrite(
    "die Anzeige der Prüfliste",
    readQueue,
  );

  const { clear: clearAfterDecideError } = afterDecide;
  useLayoutEffect(() => {
    latestLoad.current = load;
  });

  const invalidateRequest = useCallback(() => {
    latestRequest.current++;
  }, []);
  useLayoutEffect(() => {
    activePhase.current = phaseID;
    completed.current.clear();
    setItems([]);
    setClassOverrides({});
    setBusyChild(null);
    clearAfterDecideError();
    void load();
    return invalidateRequest;
  }, [phaseID, load, invalidateRequest, clearAfterDecideError]);

  const decide = async (
    item: ReviewQueueItem,
    choice: "keep" | "drop" | "defer",
  ) => {
    if (completed.current.has(item.child_id)) return;
    decision.clearFieldErrors();
    const overrideRaw = classOverrides[item.child_id]?.trim() ?? "";
    const overrideNumber =
      choice === "keep" && overrideRaw ? Number(overrideRaw) : undefined;
    if (
      choice === "keep" &&
      overrideRaw &&
      (!Number.isFinite(overrideNumber) || !Number.isInteger(overrideNumber))
    ) {
      // Lokale Prüfung: der Hinweis steht am Feld, der Fokus springt hin.
      decision.setFieldErrors({
        [gradeFieldName(item.child_id)]:
          "Bitte geben Sie die Klassenstufe als ganze Zahl ein.",
      });
      return;
    }
    setBusyChild(item.child_id);
    try {
      await decideRolloverReview(item.child_id, {
        decision: choice,
        new_grade_level: overrideNumber ?? null,
      });
    } catch (err) {
      logger.error("review_queue_decide_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await decision.show(err, {
        object: "die Entscheidung",
        retry: () => void latestDecide.current(item, choice),
      });
      if (activePhase.current === phaseID) setBusyChild(null);
      return;
    }
    if (activePhase.current === phaseID) {
      if (choice !== "defer") completed.current.add(item.child_id);
      setItems((current) =>
        current.filter((entry) => entry.child_id !== item.child_id),
      );
      setBusyChild(null);
    }
    toast.success(
      choice === "keep"
        ? "Das Kind wird in die nächste Phase übernommen."
        : choice === "drop"
          ? "Der Eintrag ist abgeschlossen."
          : "Der Eintrag ist zurückgestellt.",
    );
    refreshPhaseExpiryWarnings();
    await afterDecide.refresh();
  };
  // „Wiederholen“ sendet die Entscheidung mit der aktuellen Eingabe.
  const latestDecide = useRef(decide);
  useLayoutEffect(() => {
    latestDecide.current = decide;
  });

  if (
    loading ||
    (loadFailed && items.length === 0 && !listLoad.error && !afterDecide.error)
  ) {
    // Der Titel ist statisch und steht deshalb schon während des Ladens.
    return (
      <TenantPage
        title="Prüfliste"
        back
        backHref="/enrollment-phases"
        backLabel="Zurück zu den Anmeldephasen"
        statsLoading
        loading
      />
    );
  }

  return (
    <TenantPage
      title="Prüfliste"
      back
      backHref="/enrollment-phases"
      backLabel="Zurück zu den Anmeldephasen"
      stats={
        listLoad.error || accessRefused
          ? undefined
          : `${items.length} ${items.length === 1 ? "offener Eintrag" : "offene Einträge"}`
      }
      // Ein Ladefehler steht an Stelle der Liste, nie ein Leerzustand.
      error={listLoad.error ?? (accessRefused ? afterDecide.error : null)}
      empty={
        items.length === 0 && !accessRefused && !afterDecide.error
          ? {
              title: "Keine offenen Einträge",
              description:
                "Alle Anmeldungen wurden entweder übernommen oder bereits entschieden.",
            }
          : null
      }
    >
      <LoadErrorAlert error={afterDecide.error} className="mb-4" />
      {/* Der Erklärsatz steht auf einer Fläche, nicht frei auf dem Grund. */}
      <SectionCard className="px-5 py-3">
        <p className="text-sm leading-6 text-gray-600">
          {REVIEW_QUEUE_DESCRIPTION}
        </p>
      </SectionCard>

      {items.length === 0 ? null : (
        <ul className="space-y-3">
          {items.map((item) => {
            const reasonLabel =
              item.review_reason && REVIEW_REASON_LABELS[item.review_reason]
                ? REVIEW_REASON_LABELS[item.review_reason]
                : item.review_reason;
            const busy = busyChild === item.child_id;
            const overrideValue = classOverrides[item.child_id] ?? "";
            return (
              <li key={item.child_id}>
                <SectionCard>
                  {/* Titel links, Entscheidungen rechts: die drei Aktionen
                    hatten vorher eine eigene Zeile am Fuß des Eintrags. */}
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div className="min-w-0">
                      <h3 className="text-base font-semibold text-gray-900">
                        {item.first_name} {item.last_name}
                      </h3>
                      <p className="text-xs text-gray-600">
                        {item.guardian_first_name} {item.guardian_last_name} ·{" "}
                        {item.guardian_email}
                      </p>
                      <p className="mt-1 text-xs text-gray-500">
                        Vorgemerkt: Klasse {item.target_grade_level ?? "–"}
                        {item.source_grade_level && (
                          <> (vorher: Klasse {item.source_grade_level})</>
                        )}
                      </p>
                    </div>
                    <div className="flex shrink-0 flex-wrap items-center gap-2">
                      <Button
                        type="button"
                        size="md"
                        variant="success"
                        onClick={() => void decide(item, "keep")}
                        disabled={busy}
                      >
                        Behalten
                      </Button>
                      <Button
                        type="button"
                        size="md"
                        variant="outline_danger"
                        onClick={() => void decide(item, "drop")}
                        disabled={busy}
                      >
                        Abschließen
                      </Button>
                      <Button
                        type="button"
                        size="md"
                        variant="outline"
                        onClick={() => void decide(item, "defer")}
                        disabled={busy}
                      >
                        Zurückstellen
                      </Button>
                    </div>
                  </div>

                  {reasonLabel && (
                    <div className="mt-3">
                      <StatusColorBadge
                        color={LOCATION_COLORS.WARNING}
                        wrap
                        label={reasonLabel}
                      />
                    </div>
                  )}

                  <div className="mt-4 max-w-xs">
                    <Input
                      id={`grade-${item.child_id}`}
                      label="Klassenstufe für nächste Phase (optional)"
                      name={gradeFieldName(item.child_id)}
                      error={decision.fieldError(gradeFieldName(item.child_id))}
                      type="number"
                      controlSize="compact"
                      min={1}
                      max={MAX_SUPPORTED_GRADE_LEVEL}
                      value={overrideValue}
                      onChange={(e) =>
                        setClassOverrides((prev) => ({
                          ...prev,
                          [item.child_id]: e.target.value,
                        }))
                      }
                      placeholder={
                        item.target_grade_level
                          ? String(item.target_grade_level)
                          : ""
                      }
                      className="max-w-24"
                      disabled={busy}
                    />
                  </div>
                </SectionCard>
              </li>
            );
          })}
        </ul>
      )}
    </TenantPage>
  );
}
