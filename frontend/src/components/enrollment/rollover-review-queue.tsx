"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
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
import { createLogger } from "~/lib/logger";
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
  const toast = useToast();
  const [items, setItems] = useState<ReviewQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
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

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const list = await listRolloverReview(phaseID);
      setItems(list);
      clearLoadError();
    } catch (err) {
      logger.error("review_queue_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Prüfliste",
        retry: () => void latestLoad.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [phaseID, showLoadError, clearLoadError]);

  useLayoutEffect(() => {
    latestLoad.current = load;
  });

  useEffect(() => {
    void load();
  }, [load]);

  const decide = async (
    item: ReviewQueueItem,
    choice: "keep" | "drop" | "defer",
  ) => {
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
      toast.success(
        choice === "keep"
          ? "Das Kind wird in die nächste Phase übernommen."
          : choice === "drop"
            ? "Der Eintrag ist abgeschlossen."
            : "Der Eintrag ist zurückgestellt.",
      );
      await load();
    } catch (err) {
      logger.error("review_queue_decide_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await decision.show(err, {
        object: "die Entscheidung",
        retry: () => void latestDecide.current(item, choice),
      });
    } finally {
      setBusyChild(null);
    }
  };
  // „Wiederholen“ sendet die Entscheidung mit der aktuellen Eingabe.
  const latestDecide = useRef(decide);
  useLayoutEffect(() => {
    latestDecide.current = decide;
  });

  if (loading) {
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
        listLoad.error
          ? undefined
          : `${items.length} ${items.length === 1 ? "offener Eintrag" : "offene Einträge"}`
      }
      // Ein Ladefehler steht an Stelle der Liste, nie ein Leerzustand.
      error={listLoad.error}
      empty={
        items.length === 0
          ? {
              title: "Keine offenen Einträge",
              description:
                "Alle Anmeldungen wurden entweder übernommen oder bereits entschieden.",
            }
          : null
      }
    >
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
              <li
                key={item.child_id}
                className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6"
              >
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
                  <div className="bg-moto-amber/15 text-moto-amber-strong mt-3 inline-flex items-center rounded-full px-3 py-1 text-xs font-medium">
                    {reasonLabel}
                  </div>
                )}

                <div className="mt-4 flex flex-wrap items-center gap-3">
                  <label
                    htmlFor={`grade-${item.child_id}`}
                    className="text-xs font-semibold text-gray-700"
                  >
                    Klassenstufe für nächste Phase (optional):
                  </label>
                  <Input
                    id={`grade-${item.child_id}`}
                    name={gradeFieldName(item.child_id)}
                    error={decision.fieldError(gradeFieldName(item.child_id))}
                    type="number"
                    controlSize="compact"
                    min={1}
                    max={13}
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
                    className="w-24"
                    disabled={busy}
                  />
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </TenantPage>
  );
}
