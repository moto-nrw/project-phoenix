"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import {
  ChevronLeft,
  ChevronRight,
  Copy,
  ClipboardPaste,
  Plus,
  Trash2,
  X,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { TenantPage } from "~/components/ui/tenant-page";
import { SectionCard } from "~/components/ui/section-card";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useToast } from "~/contexts/ToastContext";
import { isoWeekNumber, parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  getMealPlanWeek,
  mondayOf,
  setDay,
  workWeekDates,
  type MealPlanEntry,
} from "~/lib/meal-plan-api";
import { createLogger } from "~/lib/logger";
import { useNavigationGuard } from "~/lib/hooks/use-navigation-guard";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "MealPlanPage" });

// Borderless fields that live inside a dish card (the card is the visual
// boundary, so the textareas stay clean — no box-in-box). field-sizing:content
// lets them grow so long dish names wrap instead of being clipped.
const dishFieldClass =
  "block w-full resize-none border-0 bg-transparent p-0 text-base leading-snug font-medium text-gray-900 placeholder:font-normal placeholder:text-gray-400 focus:ring-0 focus:outline-none [field-sizing:content]";
const noteFieldClass =
  "mt-1.5 block w-full resize-none border-0 bg-transparent p-0 text-sm leading-snug text-gray-500 placeholder:text-gray-400 focus:ring-0 focus:outline-none [field-sizing:content]";

interface DishDraft {
  clientId?: string;
  dish: string;
  note: string;
}

function createDishDraft(dish = "", note = ""): DishDraft {
  return { clientId: crypto.randomUUID(), dish, note };
}

function weekdayLabel(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", { weekday: "long" });
}

function shortDate(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
  });
}

// Day.month.year in German (e.g. "28.12.2026"). Used for the cross-year range
// label where each endpoint needs its own year; appending a year to shortDate()
// would double the trailing dot ("28.12." + ".2026" → "28.12..2026").
function shortDateWithYear(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

// Normalised, comparable form of a day: blank dishes dropped, fields trimmed.
function normalizeDay(rows: DishDraft[]): DishDraft[] {
  return rows
    .map((r) => ({ dish: r.dish.trim(), note: r.note.trim() }))
    .filter((r) => r.dish !== "");
}

function dayHasContent(rows: DishDraft[] | undefined): boolean {
  return normalizeDay(rows ?? []).length > 0;
}

// Build editable drafts (always at least one empty row per day) from persisted
// entries keyed by date.
function draftsFromEntries(
  entries: MealPlanEntry[],
  dates: string[],
): Record<string, DishDraft[]> {
  const byDate = new Map<string, DishDraft[]>();
  for (const e of entries) {
    const rows = byDate.get(e.date) ?? [];
    rows.push(createDishDraft(e.dish, e.note ?? ""));
    byDate.set(e.date, rows);
  }
  const next: Record<string, DishDraft[]> = {};
  for (const date of dates) {
    next[date] = byDate.get(date) ?? [createDishDraft()];
  }
  return next;
}

export default function MealPlanPage() {
  const toast = useToast();
  const router = useTenantRouter();
  // Gate the initial fetch on session readiness: sessionFetch needs the
  // client-side token, which isn't available for a tick on a hard reload /
  // direct navigation. Without this gate the one-shot load races auth, throws
  // "No authentication token available", and never retries — leaving the page
  // permanently empty. `required` redirects an absent/expired session to login
  // instead of hanging on the loading skeleton. Navigate via the tenant router
  // rather than next/navigation's redirect(), which isn't supported from this
  // client hook callback (it throws NEXT_REDIRECT instead of navigating).
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  // The page is reachable with config:read (admin / config-managers), but
  // writing a day requires config:update (the backend PUT/DELETE routes 403
  // otherwise). Read-only config users get a view-only plan: no edit, copy,
  // paste, clear, add/remove, "Vorwoche übernehmen", or save controls — so they
  // never hit a guaranteed 403. Mirror the nav's admin-OR-permission gate.
  const canEdit = isAdmin(session) || hasPermission(session, "config:update");

  // weekOffset: number of weeks from the current week (0 = this week, can go
  // negative for past weeks or beyond +1 for planning ahead).
  const [weekOffset, setWeekOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  // True once the first week has loaded. Subsequent week switches keep the
  // current grid on screen (dimmed) instead of flashing the skeleton.
  const [hasLoaded, setHasLoaded] = useState(false);
  // True when the displayed week failed to load. The grid is then NOT shown:
  // editing/saving against an unknown persisted state could overwrite real
  // meals based on a failed load, so we surface an error + retry instead.
  const [loadError, setLoadError] = useState(false);
  const [saving, setSaving] = useState(false);
  const [copyingPrev, setCopyingPrev] = useState(false);
  const [drafts, setDrafts] = useState<Record<string, DishDraft[]>>({});
  const [originals, setOriginals] = useState<Record<string, DishDraft[]>>({});
  const [clipboard, setClipboard] = useState<DishDraft[] | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{
    date: string;
    idx: number;
  } | null>(null);
  // Holds the week the user wants to switch to while unsaved changes block it.
  const [pendingOffset, setPendingOffset] = useState<number | null>(null);
  const [confirmCopyPrev, setConfirmCopyPrev] = useState(false);

  // Berlin-anchored: meal-plan rows are stored as school calendar DATEs the
  // backend derives from timezone.TodayDate(). A browser in another timezone
  // (or around the Berlin/local midnight boundary) would otherwise open and
  // save the wrong school week. Matches the parent meal-plan page.
  // useBerlinToday re-renders on the Berlin midnight rollover so a page left
  // mounted overnight follows the date instead of freezing on yesterday.
  const today = useBerlinToday();

  const mondayISO = useMemo(() => {
    const base = parseISODate(today);
    base.setDate(base.getDate() + weekOffset * 7);
    return mondayOf(base);
  }, [today, weekOffset]);

  const weekDates = useMemo(() => workWeekDates(mondayISO), [mondayISO]);

  // Latest-wins guard: every load() captures a token; a slower earlier load
  // (e.g. a previous week on a slow connection) bails instead of overwriting
  // the newer week's drafts/originals after it resolves. A counter rather than
  // a per-effect cancelled flag because load() is also called by the retry
  // button and after save, not only from the week-switch effect.
  const loadTokenRef = useRef(0);

  const load = useCallback(async () => {
    const token = ++loadTokenRef.current;
    setLoading(true);
    setLoadError(false);
    try {
      const entries = await getMealPlanWeek(mondayISO);
      if (loadTokenRef.current !== token) return;
      const built = draftsFromEntries(entries, weekDates);
      setDrafts(built);
      // Originals store only the persisted (non-empty) rows for dirty checks.
      const orig: Record<string, DishDraft[]> = {};
      for (const date of weekDates) orig[date] = normalizeDay(built[date]!);
      setOriginals(orig);
    } catch (err) {
      if (loadTokenRef.current !== token) return;
      logger.error("meal_plan_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Operational failure: drop any stale week so nothing editable is shown
      // against an unknown persisted state, and surface an error instead.
      setDrafts({});
      setOriginals({});
      setLoadError(true);
      toast.error("Essensplan konnte nicht geladen werden.");
    } finally {
      // Only the most recent load owns the shared loading/loaded flags.
      if (loadTokenRef.current === token) {
        setLoading(false);
        setHasLoaded(true);
      }
    }
  }, [mondayISO, weekDates, toast]);

  useEffect(() => {
    if (status !== "authenticated") return;
    void load();
  }, [load, status]);

  const changedDays = useMemo(
    () =>
      weekDates.filter(
        (date) =>
          JSON.stringify(normalizeDay(drafts[date] ?? [])) !==
          JSON.stringify(originals[date] ?? []),
      ),
    [weekDates, drafts, originals],
  );
  const isDirty = changedDays.length > 0;

  // Freeze all edit controls while a save, week-load, or "Vorwoche übernehmen"
  // is in flight. handleSave() captures the pre-click draft snapshot and load()
  // then replaces drafts/originals from the backend; an edit made after the
  // request started would otherwise be silently overwritten when it resolves
  // (same for an edit made while a new week is still loading). canEdit gates
  // *who* may edit; editingLocked gates *when*.
  const editingLocked = saving || copyingPrev || loading;
  const canEditNow = canEdit && !editingLocked;

  // Warn before discarding unsaved changes — both on hard unloads (tab close /
  // reload) and on in-app navigation (clicking a sidebar/header link), which
  // does a client-side route change that beforeunload never sees.
  const { pendingHref, confirmNavigation, cancelNavigation } =
    useNavigationGuard(isDirty);

  const updateDish = (
    date: string,
    idx: number,
    field: keyof DishDraft,
    value: string,
  ) => {
    setDrafts((prev) => {
      const rows = [...(prev[date] ?? [])];
      rows[idx] = { ...rows[idx]!, [field]: value };
      return { ...prev, [date]: rows };
    });
  };

  const addRow = (date: string) => {
    setDrafts((prev) => ({
      ...prev,
      [date]: [...(prev[date] ?? []), createDishDraft()],
    }));
  };

  const removeRow = (date: string, idx: number) => {
    setDrafts((prev) => {
      const rows = (prev[date] ?? []).filter((_, i) => i !== idx);
      return { ...prev, [date]: rows.length ? rows : [createDishDraft()] };
    });
  };

  // A dish with content goes through a confirmation; an empty row is removed
  // straight away (nothing to lose).
  const requestRemove = (date: string, idx: number) => {
    const row = drafts[date]?.[idx];
    if (row && (row.dish.trim() !== "" || row.note.trim() !== "")) {
      setDeleteTarget({ date, idx });
    } else {
      removeRow(date, idx);
    }
  };

  const confirmRemove = () => {
    if (deleteTarget) removeRow(deleteTarget.date, deleteTarget.idx);
    setDeleteTarget(null);
  };

  // --- Per-day copy / paste / clear -------------------------------------
  const copyDay = (date: string) => {
    setClipboard(normalizeDay(drafts[date] ?? []).map((r) => ({ ...r })));
    toast.success(`${weekdayLabel(date)} kopiert.`);
  };

  const pasteDay = (date: string) => {
    if (!clipboard) return;
    const rows = clipboard.length
      ? clipboard.map((row) => createDishDraft(row.dish, row.note))
      : [createDishDraft()];
    setDrafts((prev) => ({ ...prev, [date]: rows }));
  };

  const clearDay = (date: string) => {
    setDrafts((prev) => ({ ...prev, [date]: [createDishDraft()] }));
  };

  // --- Week navigation (guarded against unsaved changes) ----------------
  const attemptWeekChange = (next: number) => {
    if (next === weekOffset) return;
    // Block week switches while a save (or prev-week copy) is in flight. The
    // post-save reload is bound to the week that was on screen when Save was
    // clicked; switching mid-save would let that stale reload win the
    // latest-wins token race and write the old week's data over the new week,
    // leaving an unloaded/blank grid that the next edit could overwrite.
    if (saving || copyingPrev) return;
    if (isDirty) {
      setPendingOffset(next);
    } else {
      setWeekOffset(next);
    }
  };

  const confirmWeekChange = () => {
    if (pendingOffset !== null) setWeekOffset(pendingOffset);
    setPendingOffset(null);
  };

  // --- Copy the previous week's plan into the current week ---------------
  const doCopyPreviousWeek = useCallback(async () => {
    setCopyingPrev(true);
    try {
      const prevMonday = (() => {
        const base = parseISODate(mondayISO);
        base.setDate(base.getDate() - 7);
        return toISODate(base);
      })();
      const prevDates = workWeekDates(prevMonday);
      const entries = await getMealPlanWeek(prevMonday);
      const byDate = new Map<string, DishDraft[]>();
      for (const e of entries) {
        const rows = byDate.get(e.date) ?? [];
        rows.push(createDishDraft(e.dish, e.note ?? ""));
        byDate.set(e.date, rows);
      }
      setDrafts((prev) => {
        const next = { ...prev };
        prevDates.forEach((pd, i) => {
          const target = weekDates[i]!;
          const rows = byDate.get(pd);
          next[target] = rows?.length
            ? rows.map((row) => createDishDraft(row.dish, row.note))
            : [createDishDraft()];
        });
        return next;
      });
      toast.success("Vorwoche übernommen. Noch nicht gespeichert.");
    } catch (err) {
      logger.error("meal_plan_copy_prev_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Vorwoche konnte nicht geladen werden.");
    } finally {
      setCopyingPrev(false);
    }
  }, [mondayISO, weekDates, toast]);

  const requestCopyPreviousWeek = () => {
    if (weekDates.some((d) => dayHasContent(drafts[d]))) {
      setConfirmCopyPrev(true);
    } else {
      void doCopyPreviousWeek();
    }
  };

  // --- Save / discard ---------------------------------------------------
  const discard = () => {
    setDrafts((prev) => {
      const next = { ...prev };
      for (const date of weekDates) {
        const o = originals[date] ?? [];
        next[date] = o.length
          ? o.map((row) => createDishDraft(row.dish, row.note))
          : [createDishDraft()];
      }
      return next;
    });
  };

  const handleSave = async () => {
    setSaving(true);
    // Each setDay is its own backend transaction, so a mid-loop failure leaves
    // the already-sent days committed. Track what actually persisted and fold
    // it into originals on failure, so the dirty indicator and Verwerfen reflect
    // what is really stored rather than the pre-save snapshot.
    const persisted: Record<string, DishDraft[]> = {};
    try {
      for (const date of weekDates) {
        const draft = normalizeDay(drafts[date] ?? []);
        const original = originals[date] ?? [];
        if (JSON.stringify(draft) === JSON.stringify(original)) continue;
        await setDay(
          date,
          draft.map((d) => ({
            dish: d.dish,
            note: d.note === "" ? null : d.note,
          })),
        );
        persisted[date] = draft;
      }
      toast.success("Essensplan gespeichert.");
      await load();
    } catch (err) {
      logger.error("meal_plan_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Commit the days that did persist; the still-unsaved days stay dirty.
      if (Object.keys(persisted).length > 0) {
        setOriginals((prev) => ({ ...prev, ...persisted }));
      }
      toast.error("Speichern fehlgeschlagen.");
    } finally {
      setSaving(false);
    }
  };

  const weekNumber = isoWeekNumber(mondayISO);
  // A work week can straddle New Year (e.g. 28.12.2026 – 01.01.2027). Only
  // append a single trailing year when both endpoints share it; otherwise label
  // each endpoint with its own year so the range is unambiguous.
  const startYear = parseISODate(weekDates[0]!).getFullYear();
  const endYear = parseISODate(weekDates[4]!).getFullYear();
  const rangeLabel =
    startYear === endYear
      ? `${shortDate(weekDates[0]!)} – ${shortDate(weekDates[4]!)} ${startYear}`
      : `${shortDateWithYear(weekDates[0]!)} – ${shortDateWithYear(weekDates[4]!)}`;
  const isCurrentWeek = weekOffset === 0;

  // Statuszeile unter dem Titel: Kalenderwoche, Zeitraum und wie viele der
  // fünf Wochentage schon ein Gericht haben. Die Zahl kommt aus den bereits
  // gespeicherten Tagen der geladenen Woche.
  const plannedDays = weekDates.filter(
    (date) => (originals[date]?.length ?? 0) > 0,
  ).length;
  const spokenRange =
    startYear === endYear
      ? `${shortDate(weekDates[0]!)} bis ${shortDate(weekDates[4]!)}${startYear}`
      : `${shortDateWithYear(weekDates[0]!)} bis ${shortDateWithYear(weekDates[4]!)}`;
  const initialLoading = loading && !hasLoaded;
  const statusLine = `KW ${weekNumber} · ${spokenRange} · ${plannedDays} von ${weekDates.length} Tagen geplant`;

  return (
    <>
      <TenantPage
        title="Essensplan"
        stats={statusLine}
        statsLoading={initialLoading}
        loading={initialLoading}
        error={
          loadError
            ? {
                message:
                  "Essensplan konnte nicht geladen werden. Bitte versuchen Sie es erneut.",
                action: (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => void load()}
                    disabled={loading}
                  >
                    Erneut versuchen
                  </Button>
                ),
              }
            : null
        }
      >
        {/* Erklärtext als description, „Vorwoche übernehmen“ in der Titelzeile:
            kein frei stehender Absatz und keine Zeile nur aus Buttons. */}
        <SectionCard
          title="Wochenplan"
          description="Pro Tag ein oder mehrere Gerichte mit optionalem Hinweis. Eltern sehen den Plan im Elternportal."
          actions={
            <>
              {/* Wochennavigation steht in der Titelzeile, damit die Karte
                  nicht aus zwei fast leeren Zeilen besteht. */}
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Vorherige Woche"
                  onClick={() => attemptWeekChange(weekOffset - 1)}
                  disabled={loading || saving || copyingPrev}
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <div className="min-w-32 text-center">
                  <div className="text-sm font-semibold text-gray-900">
                    KW {weekNumber}
                    {isCurrentWeek ? (
                      <span className="text-moto-green ml-1.5">
                        · Diese Woche
                      </span>
                    ) : null}
                  </div>
                  <div className="text-xs text-gray-500">{rangeLabel}</div>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Nächste Woche"
                  onClick={() => attemptWeekChange(weekOffset + 1)}
                  disabled={loading || saving || copyingPrev}
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
                {!isCurrentWeek && (
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => attemptWeekChange(0)}
                    disabled={loading || saving || copyingPrev}
                  >
                    Heute
                  </Button>
                )}
              </div>
              {canEdit ? (
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={requestCopyPreviousWeek}
                  disabled={loading || saving || copyingPrev || loadError}
                  isLoading={copyingPrev}
                  loadingText="Übernehmen…"
                >
                  <Copy className="mr-2 h-4 w-4" />
                  Vorwoche übernehmen
                </Button>
              ) : null}
            </>
          }
        />

        <SectionCard
          className={`transition-opacity duration-200 ${
            loading ? "opacity-50" : "opacity-100"
          }`}
          bodyClassName=""
        >
          <div className="grid grid-cols-1 divide-y divide-gray-200 md:grid-cols-5 md:divide-x md:divide-y-0">
            {weekDates.map((date) => {
              const rows = drafts[date] ?? [];
              const isToday = date === today;
              const hasContent = dayHasContent(rows);
              return (
                <div
                  key={date}
                  data-meal-day-column
                  className={`flex flex-col ${isToday ? "bg-moto-green/[0.04]" : ""}`}
                >
                  <div
                    className={`flex items-center justify-between gap-2 border-b px-4 py-3 ${
                      isToday ? "border-moto-green/30" : "border-gray-200"
                    }`}
                  >
                    <div>
                      <div className="text-sm font-semibold text-gray-900">
                        {weekdayLabel(date)}
                      </div>
                      <div className="text-xs text-gray-500">
                        {shortDate(date)}
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      {isToday && (
                        <span className="bg-moto-green rounded-full px-2 py-0.5 text-[11px] font-semibold text-gray-950">
                          Heute
                        </span>
                      )}
                      {canEdit && (
                        <OverflowMenu
                          ariaLabel={`Aktionen für ${weekdayLabel(date)}`}
                          triggerClassName="!size-7"
                          matchContainerSelector="[data-meal-day-column]"
                          items={[
                            {
                              label: "Tag kopieren",
                              icon: <Copy className="h-4 w-4" />,
                              onClick: () => copyDay(date),
                              disabled: !hasContent || editingLocked,
                            },
                            {
                              label: "Einfügen",
                              icon: <ClipboardPaste className="h-4 w-4" />,
                              onClick: () => pasteDay(date),
                              disabled: clipboard === null || editingLocked,
                            },
                            {
                              label: "Tag leeren",
                              icon: <Trash2 className="h-4 w-4" />,
                              onClick: () => clearDay(date),
                              destructive: true,
                              disabled: !hasContent || editingLocked,
                            },
                          ]}
                        />
                      )}
                    </div>
                  </div>

                  <div className="flex flex-1 flex-col gap-3 p-3">
                    {rows.map((row, idx) => (
                      <div
                        key={row.clientId}
                        className="group relative min-h-20 rounded-lg border border-gray-200 bg-white p-3.5 pr-8 transition focus-within:border-gray-300 focus-within:ring-1 focus-within:ring-gray-200"
                      >
                        <textarea
                          rows={1}
                          className={dishFieldClass}
                          value={row.dish}
                          placeholder="Gericht eintragen…"
                          readOnly={!canEditNow}
                          onChange={(e) =>
                            updateDish(date, idx, "dish", e.target.value)
                          }
                        />
                        <textarea
                          rows={1}
                          className={noteFieldClass}
                          value={row.note}
                          placeholder="Hinweis (optional)"
                          readOnly={!canEditNow}
                          onChange={(e) =>
                            updateDish(date, idx, "note", e.target.value)
                          }
                        />
                        {canEdit &&
                          (rows.length > 1 ||
                            row.dish.trim() !== "" ||
                            row.note.trim() !== "") && (
                            <button
                              type="button"
                              aria-label="Gericht entfernen"
                              onClick={() => requestRemove(date, idx)}
                              disabled={editingLocked}
                              className="absolute top-2 right-2 rounded p-1 text-gray-400 transition hover:bg-gray-100 hover:text-gray-600 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-gray-400"
                            >
                              <X className="h-4 w-4" />
                            </button>
                          )}
                      </div>
                    ))}

                    {canEdit && (
                      <button
                        type="button"
                        onClick={() => addRow(date)}
                        disabled={editingLocked}
                        className="flex items-center justify-center gap-1 rounded-lg border border-dashed border-gray-300 py-2 text-sm font-medium text-gray-500 transition hover:border-gray-400 hover:bg-gray-50 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-gray-300 disabled:hover:bg-transparent disabled:hover:text-gray-500"
                      >
                        <Plus className="h-4 w-4" />
                        Gericht
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </SectionCard>

        {/* Sticky save bar — only while there are unsaved changes. */}
        {canEdit && isDirty && !loading && !loadError && (
          <div className="sticky bottom-4 z-20">
            <SectionCard className="shadow-lg" bodyClassName="">
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium text-gray-700">
                  {changedDays.length}{" "}
                  {changedDays.length === 1 ? "Tag geändert" : "Tage geändert"}
                </span>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={discard}
                    disabled={saving}
                  >
                    Verwerfen
                  </Button>
                  <Button
                    type="button"
                    variant="primary"
                    size="md"
                    onClick={handleSave}
                    disabled={saving}
                    isLoading={saving}
                    loadingText="Speichern…"
                  >
                    Speichern
                  </Button>
                </div>
              </div>
            </SectionCard>
          </div>
        )}
      </TenantPage>

      {/* Löschen läuft portalweit über dasselbe Bauteil. */}
      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Gericht entfernen"
        description={
          deleteTarget
            ? `„${drafts[deleteTarget.date]?.[deleteTarget.idx]?.dish.trim() || "Dieses Gericht"}“ wird aus dem Plan entfernt. Die Änderung wird mit „Speichern“ übernommen.`
            : ""
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Entfernen"
        onConfirm={confirmRemove}
        onClose={() => setDeleteTarget(null)}
        loading={false}
        error=""
      />

      <ConfirmationModal
        isOpen={pendingOffset !== null}
        onClose={() => setPendingOffset(null)}
        onConfirm={confirmWeekChange}
        title="Ungespeicherte Änderungen"
        confirmText="Verwerfen & wechseln"
        cancelText="Hierbleiben"
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      >
        <p className="text-sm text-gray-600">
          Sie haben Änderungen in dieser Woche, die noch nicht gespeichert sind.
          Beim Wochenwechsel gehen sie verloren.
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={pendingHref !== null}
        onClose={cancelNavigation}
        onConfirm={confirmNavigation}
        title="Ungespeicherte Änderungen"
        confirmText="Verwerfen & verlassen"
        cancelText="Hierbleiben"
        confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      >
        <p className="text-sm text-gray-600">
          Sie haben Änderungen, die noch nicht gespeichert sind. Beim Verlassen
          der Seite gehen sie verloren.
        </p>
      </ConfirmationModal>

      <ConfirmationModal
        isOpen={confirmCopyPrev}
        onClose={() => setConfirmCopyPrev(false)}
        onConfirm={() => {
          setConfirmCopyPrev(false);
          void doCopyPreviousWeek();
        }}
        title="Vorwoche übernehmen?"
        confirmText="Übernehmen"
        cancelText="Abbrechen"
      >
        <p className="text-sm text-gray-600">
          Die aktuelle Woche wird mit dem Plan der Vorwoche überschrieben. Die
          Änderung wird erst mit „Speichern“ übernommen.
        </p>
      </ConfirmationModal>
    </>
  );
}
