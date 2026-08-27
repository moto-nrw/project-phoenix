"use client";

import { useCallback, useEffect, useState } from "react";
import {
  decideRolloverReview,
  listRolloverReview,
  type ReviewQueueItem,
} from "~/lib/enrollment-phase-api";
import { createLogger } from "~/lib/logger";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";

const logger = createLogger({ component: "RolloverReviewQueue" });

const REVIEW_REASON_LABELS: Record<string, string> = {
  grade_above_max: "Klassenstufe über der Höchstgrenze",
  no_grade_level: "Keine Klassenstufe hinterlegt",
};

interface Props {
  readonly phaseID: string;
  readonly phaseName?: string;
}

export function RolloverReviewQueue({ phaseID, phaseName }: Props) {
  const [items, setItems] = useState<ReviewQueueItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [busyChild, setBusyChild] = useState<string | null>(null);
  const [classOverrides, setClassOverrides] = useState<Record<string, string>>(
    {},
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listRolloverReview(phaseID);
      setItems(list);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("review_queue_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [phaseID]);

  useEffect(() => {
    void load();
  }, [load]);

  const decide = async (
    item: ReviewQueueItem,
    decision: "keep" | "drop" | "defer",
  ) => {
    setBusyChild(item.child_id);
    setError(null);
    setInfo(null);
    try {
      const overrideRaw = classOverrides[item.child_id]?.trim() ?? "";
      const overrideNumber =
        decision === "keep" && overrideRaw ? Number(overrideRaw) : undefined;
      if (
        decision === "keep" &&
        overrideRaw &&
        (!Number.isFinite(overrideNumber) || !Number.isInteger(overrideNumber))
      ) {
        throw new Error("Klassenstufe muss eine ganze Zahl sein.");
      }
      await decideRolloverReview(item.child_id, {
        decision,
        new_grade_level: overrideNumber ?? null,
      });
      const verb =
        decision === "keep"
          ? "übernommen"
          : decision === "drop"
            ? "verworfen"
            : "zurückgestellt";
      setInfo(`Eintrag ${verb}.`);
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("review_queue_decide_failed", { error: message });
      setError(message);
    } finally {
      setBusyChild(null);
    }
  };

  if (loading) {
    return (
      <SkeletonRegion label="Prüfliste wird geladen">
        <ListSkeleton rows={4} avatar={false} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="space-y-4">
      <header>
        {phaseName ? (
          <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            {phaseName}
          </p>
        ) : null}
        <p className="mt-1 text-sm text-gray-600">
          Kinder, die nicht automatisch übernommen werden konnten, meist weil
          ihre Klassenstufe nach Erhöhung über der Höchstgrenze liegt. Wählen
          Sie pro Eintrag, ob Sie das Kind behalten (ggf. mit anderer
          Klassenstufe), aus der nächsten Phase entfernen oder vorerst
          zurückstellen.
        </p>
      </header>

      {error ? <Alert type="error" message={error} /> : null}
      {info ? <Alert type="success" message={info} /> : null}

      {items.length === 0 ? (
        <EmptyState
          title="Keine offenen Einträge"
          description="Alle Anmeldungen wurden entweder übernommen oder bereits entschieden."
        />
      ) : (
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
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <div>
                    <h3 className="text-base font-semibold text-gray-900">
                      {item.first_name} {item.last_name}
                    </h3>
                    <p className="text-xs text-gray-600">
                      {item.guardian_first_name} {item.guardian_last_name} ·{" "}
                      {item.guardian_email}
                    </p>
                  </div>
                  <div className="text-xs text-gray-500">
                    Vorgemerkt: Klasse {item.target_grade_level ?? "—"}
                    {item.source_grade_level && (
                      <> (vorher: Klasse {item.source_grade_level})</>
                    )}
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

                <div className="mt-4 flex flex-wrap gap-2">
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
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
