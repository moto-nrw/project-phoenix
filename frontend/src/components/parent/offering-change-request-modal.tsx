"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";

import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { ISODatePicker } from "~/components/ui/date-picker";
import {
  getChildOfferingCatalog,
  type OfferingCatalog,
  type OfferingCatalogItem,
  type OfferingChangeSelectionInput,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { formatDate } from "~/lib/date-helpers";
import { materializeCareSelection } from "~/lib/care-offering-materialization";

const logger = createLogger({ component: "OfferingChangeRequestModal" });

// Day abbreviations as the backend stores them, in week order. The label comes
// from parentMasterData.weekdayShort keyed by ISO weekday.
const DAY_ORDER = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] as const;
const DAY_TO_ISO: Record<string, number> = {
  mon: 1,
  tue: 2,
  wed: 3,
  thu: 4,
  fri: 5,
  sat: 6,
  sun: 7,
};

interface OfferingDraft {
  selected: boolean;
  days: Set<string>;
}

type DraftMap = Record<string, OfferingDraft>;

function draftFromCatalog(items: OfferingCatalogItem[]): DraftMap {
  const draft: DraftMap = {};
  for (const item of items) {
    draft[item.id] = {
      selected: item.selected,
      days: new Set(item.selected_days),
    };
  }
  return draft;
}

function rebaseDraftForCatalog(
  draft: DraftMap,
  items: OfferingCatalogItem[],
  editedOfferingIDs: ReadonlySet<string>,
): DraftMap {
  const rebased = draftFromCatalog(items);
  for (const item of items) {
    if (!editedOfferingIDs.has(item.id)) continue;
    const previous = draft[item.id];
    if (!previous || item.automatic || item.is_required) continue;
    const canKeep =
      item.is_active !== false &&
      (item.selected || item.free_slots === undefined || item.free_slots > 0);
    if (!canKeep) continue;
    const days =
      item.days_of_week_mode === "parent_choice"
        ? new Set(
            [...previous.days].filter((day) =>
              item.available_days.includes(day),
            ),
          )
        : new Set<string>();
    rebased[item.id] = {
      selected: previous.selected,
      days,
    };
  }
  return rebased;
}

/** Sorted day list so a comparison and the payload are order-independent. */
function orderedDays(days: Set<string>): string[] {
  return DAY_ORDER.filter((day) => days.has(day));
}

/**
 * The parent -> OGS "change the booked care offerings" request form. The parent
 * picks the offerings the child should attend from that date on; the submission
 * carries the COMPLETE desired booking, because that is what staff decide and
 * what the backend re-validates against capacity and the phase rules.
 */
export function OfferingChangeRequestModal({
  studentId,
  onClose,
  onSubmit,
}: Readonly<{
  studentId: string;
  onClose: () => void;
  onSubmit: (input: {
    offerings: OfferingChangeSelectionInput[];
    effective_from: string;
    note?: string;
  }) => Promise<void>;
}>) {
  const t = useTranslations("parentMasterData");
  const locale = useLocale();
  const [catalog, setCatalog] = useState<OfferingCatalog | null>(null);
  const [draft, setDraft] = useState<DraftMap>({});
  const [effectiveFrom, setEffectiveFrom] = useState("");
  const [note, setNote] = useState("");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const editedOfferingIDs = useRef(new Set<string>());
  const catalogRequestID = useRef(0);

  const load = useCallback(async () => {
    const requestID = ++catalogRequestID.current;
    setLoading(true);
    setError(null);
    try {
      const loaded = await getChildOfferingCatalog(studentId);
      if (requestID !== catalogRequestID.current) return;
      setCatalog(loaded);
      setDraft(draftFromCatalog(loaded.items));
      editedOfferingIDs.current.clear();
      setEffectiveFrom(loaded.earliest_effective_from);
    } catch (err) {
      if (requestID !== catalogRequestID.current) return;
      logger.warn("offering_catalog_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setError(t("careOfferingsModal.loadError"));
    } finally {
      if (requestID === catalogRequestID.current) setLoading(false);
    }
  }, [studentId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const loadForEffectiveDate = async (date: string) => {
    if (!date || date === effectiveFrom) return;
    const requestID = ++catalogRequestID.current;
    setLoading(true);
    setError(null);
    try {
      const loaded = await getChildOfferingCatalog(studentId, date);
      if (requestID !== catalogRequestID.current) return;
      setCatalog(loaded);
      setDraft((current) =>
        rebaseDraftForCatalog(current, loaded.items, editedOfferingIDs.current),
      );
      setEffectiveFrom(date);
    } catch (err) {
      if (requestID !== catalogRequestID.current) return;
      logger.warn("offering_catalog_reload_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
        effective_from: date,
      });
      setError(t("careOfferingsModal.loadError"));
    } finally {
      if (requestID === catalogRequestID.current) setLoading(false);
    }
  };

  const weekdayLabel = useCallback(
    (day: string) => t(`weekdayShort.${DAY_TO_ISO[day] ?? 1}`),
    [t],
  );

  const toggleOffering = (item: OfferingCatalogItem) => {
    editedOfferingIDs.current.add(item.id);
    setDraft((prev) => {
      const row = prev[item.id] ?? { selected: false, days: new Set<string>() };
      const nextSelected = !row.selected;
      // Selecting an offering with fixed days implies all of them; a
      // parent-choice offering starts from the days already booked, else empty
      // so the parent has to make the choice deliberately.
      const days =
        nextSelected && item.days_of_week_mode === "parent_choice"
          ? new Set(row.days)
          : new Set<string>();
      return { ...prev, [item.id]: { selected: nextSelected, days } };
    });
  };

  const toggleDay = (offeringId: string, day: string) => {
    editedOfferingIDs.current.add(offeringId);
    setDraft((prev) => {
      const row = prev[offeringId];
      if (!row) return prev;
      const days = new Set(row.days);
      if (days.has(day)) {
        days.delete(day);
      } else {
        days.add(day);
      }
      return { ...prev, [offeringId]: { ...row, days } };
    });
  };

  const items = useMemo(() => catalog?.items ?? [], [catalog]);
  const emptyCatalog = catalog !== null && items.length === 0;

  // Mirrors the backend materializer so parents see, before submitting, which
  // offerings (and days) a Mitbuchungs-Regel adds to their selection (#2366).
  const preview = useMemo(() => {
    const selectedIds: string[] = [];
    const offeringDays: Record<string, string[]> = {};
    for (const item of items) {
      const row = draft[item.id];
      if (!row?.selected || item.automatic) continue;
      selectedIds.push(item.id);
      if (item.days_of_week_mode === "parent_choice") {
        offeringDays[item.id] = orderedDays(row.days);
      }
    }
    // gradeLevel "" is correct here: the catalog endpoint already evaluated
    // the grade condition — trigger IDs arrive pre-filtered and
    // auto_add_applies carries the gate for the lunch derivation.
    return materializeCareSelection(
      { gradeLevel: "", offeringIds: selectedIds, offeringDays },
      items,
    );
  }, [items, draft]);

  const autoHintFor = (item: OfferingCatalogItem): string | undefined => {
    const days = preview.automaticDays[item.id];
    if (!days || days.size === 0) {
      return item.automatic ? t("careOfferings.autoAdded") : undefined;
    }
    const dayText = DAY_ORDER.filter((day) => days.has(day))
      .map((day) => weekdayLabel(day))
      .join(", ");
    const names = [...(preview.autoAddContributors[item.id] ?? [])]
      .map((id) => items.find((other) => other.id === id)?.name)
      .filter((name): name is string => Boolean(name));
    return names.length > 0
      ? t("careOfferings.autoAddedDaysWithTrigger", {
          days: dayText,
          trigger: names.join(", "),
        })
      : t("careOfferings.autoAddedDays", { days: dayText });
  };

  const handleSubmit = async () => {
    if (!catalog) return;
    const offerings: OfferingChangeSelectionInput[] = [];
    for (const item of items) {
      const row = draft[item.id];
      if (!row?.selected || item.automatic) continue;
      const manualDays = orderedDays(row.days);
      const selectedDays = orderedDays(
        new Set([...manualDays, ...(preview.automaticDays[item.id] ?? [])]),
      );
      // A parent-choice offering without a day would be booked "all days" by the
      // backend's own convention, which is not what an empty checkbox row means.
      if (
        item.days_of_week_mode === "parent_choice" &&
        selectedDays.length === 0
      ) {
        setError(t("careOfferingsModal.noDaysSelected"));
        return;
      }
      offerings.push({
        offering_id: item.id,
        selected_days:
          item.days_of_week_mode === "parent_choice" ? manualDays : [],
      });
    }
    setSubmitting(true);
    setError(null);
    try {
      await onSubmit({
        offerings,
        effective_from: effectiveFrom,
        note: note.trim() === "" ? undefined : note.trim(),
      });
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : t("careOfferingsModal.submitError"),
      );
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={t("careOfferingsModal.title")}
      mobileSheet
      footer={
        <>
          <Button
            type="button"
            variant="surface"
            size="md"
            className="hidden sm:inline-flex"
            onClick={onClose}
          >
            {t("careOfferingsModal.cancel")}
          </Button>
          <Button
            type="button"
            size="md"
            className="w-full gap-2 sm:w-auto"
            onClick={() => void handleSubmit()}
            disabled={submitting || loading || !catalog || emptyCatalog}
          >
            {submitting && (
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
            )}
            {t("careOfferingsModal.submit")}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm leading-6 text-gray-600">
          {t("careOfferingsModal.intro")}
        </p>

        {loading && (
          <div className="space-y-2">
            <div className="h-16 animate-pulse rounded-xl bg-gray-100" />
            <div className="h-16 animate-pulse rounded-xl bg-gray-100" />
          </div>
        )}

        {!loading && emptyCatalog && (
          <Alert type="info" message={t("careOfferingsModal.noOfferings")} />
        )}

        {!loading && catalog && !emptyCatalog && (
          <>
            <div className="space-y-3">
              {items.map((item) => {
                const row = draft[item.id];
                const selected = preview.offeringIds.has(item.id);
                const autoHint = autoHintFor(item);
                const full =
                  item.free_slots !== undefined &&
                  item.free_slots <= 0 &&
                  !item.selected;
                const unavailable = item.is_active === false && !item.selected;
                return (
                  <div
                    key={item.id}
                    className="rounded-xl border border-gray-200 p-3"
                  >
                    <label className="flex cursor-pointer items-start gap-3">
                      <Checkbox
                        checked={selected}
                        disabled={
                          full ||
                          unavailable ||
                          item.automatic ||
                          (item.is_required && selected)
                        }
                        onChange={() => toggleOffering(item)}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="flex flex-wrap items-center gap-2">
                          <span className="text-sm font-semibold text-gray-900">
                            {item.name}
                          </span>
                          {item.is_required && (
                            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                              {t("careOfferingsModal.requiredBadge")}
                            </span>
                          )}
                          {full && (
                            <span className="rounded-full bg-[#FF3130]/10 px-2 py-0.5 text-xs font-medium text-[#CC2626]">
                              {t("careOfferingsModal.fullBadge")}
                            </span>
                          )}
                          {!full && item.free_slots !== undefined && (
                            <span className="text-xs text-gray-500">
                              {t("careOfferingsModal.freeSlots", {
                                count: item.free_slots,
                              })}
                            </span>
                          )}
                        </span>
                        {item.description && (
                          <span className="mt-0.5 block text-xs leading-5 text-gray-500">
                            {item.description}
                          </span>
                        )}
                        {autoHint && (
                          <span className="mt-1 block text-xs leading-5 font-medium text-gray-700">
                            {autoHint}
                          </span>
                        )}
                      </span>
                    </label>

                    {selected && item.days_of_week_mode === "parent_choice" && (
                      <div className="mt-3 pl-8">
                        <p className="mb-1.5 text-xs font-medium text-gray-500">
                          {t("careOfferingsModal.daysLabel")}
                        </p>
                        <div className="flex flex-wrap gap-2">
                          {DAY_ORDER.filter((day) =>
                            item.available_days.includes(day),
                          ).map((day) => {
                            const autoDay =
                              preview.automaticDays[item.id]?.has(day) ?? false;
                            const locked = item.automatic || autoDay;
                            return (
                              <label
                                key={day}
                                className={`flex items-center gap-1.5 rounded-lg border border-gray-200 px-2.5 py-1.5 ${locked ? "cursor-not-allowed" : "cursor-pointer"}`}
                              >
                                <Checkbox
                                  checked={
                                    (row?.days.has(day) ?? false) || autoDay
                                  }
                                  disabled={locked}
                                  onChange={() => toggleDay(item.id, day)}
                                />
                                <span className="text-sm text-gray-700">
                                  {weekdayLabel(day)}
                                </span>
                              </label>
                            );
                          })}
                        </div>
                      </div>
                    )}

                    {selected && item.days_of_week_mode !== "parent_choice" && (
                      <p className="mt-2 pl-8 text-xs text-gray-500">
                        {t("careOfferingsModal.fixedDays", {
                          days: item.available_days
                            .map((day) => weekdayLabel(day))
                            .join(", "),
                        })}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>

            <div>
              <ISODatePicker
                label={t("careOfferingsModal.effectiveFromLabel")}
                value={effectiveFrom}
                min={catalog.earliest_effective_from}
                max={catalog.latest_effective_from}
                required
                disabled={loading}
                onChange={(date) => void loadForEffectiveDate(date)}
              />
              <p className="mt-1 text-xs text-gray-500">
                {t("careOfferingsModal.effectiveFromHint", {
                  date: formatDate(
                    catalog.earliest_effective_from,
                    false,
                    locale,
                  ),
                })}
              </p>
            </div>

            <label className="block">
              <span className="mb-1 block text-sm font-medium text-gray-700">
                {t("careOfferingsModal.noteLabel")}
              </span>
              <textarea
                value={note}
                onChange={(event) => setNote(event.target.value)}
                rows={3}
                maxLength={2000}
                placeholder={t("careOfferingsModal.notePlaceholder")}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus-visible:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
              />
            </label>
          </>
        )}

        {error && <Alert type="error" message={error} />}
      </div>
    </Modal>
  );
}
