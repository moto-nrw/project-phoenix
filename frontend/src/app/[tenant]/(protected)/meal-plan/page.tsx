"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { Copy, ClipboardPaste, Plus, Trash2, X } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { TenantPage } from "~/components/ui/tenant-page";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  getMealPlanWeek,
  mondayOf,
  setDay,
  workWeekDates,
  type MealPlanEntry,
} from "~/lib/meal-plan-api";
import { createLogger } from "~/lib/logger";
import { formatWeekLabel } from "~/lib/timetable-helpers";
import { useNavigationGuard } from "~/lib/hooks/use-navigation-guard";
import {
  useMealPlanEnabled,
  useMealRegistrationEnabled,
} from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";
import { MealParticipantList } from "~/components/meal-plan/meal-participant-list";
import { MealCaterer } from "~/components/meal-plan/meal-caterer";
import { MealOrderWeek } from "~/components/meal-plan/meal-order-week";
import {
  MealAllergenLegend,
  MealAllergenMarks,
  MealAllergenSelect,
} from "~/components/meal-plan/meal-allergens";
import {
  normalizeMealAllergens,
  usedMealAllergens,
  type MealAllergen,
} from "~/lib/meal-allergens";

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
  allergens: MealAllergen[];
}

function createDishDraft(
  dish = "",
  note = "",
  allergens: readonly MealAllergen[] = [],
): DishDraft {
  return {
    clientId: crypto.randomUUID(),
    dish,
    note,
    allergens: [...allergens],
  };
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

// Normalised, comparable form of a day: blank dishes dropped, fields trimmed.
function normalizeDay(rows: DishDraft[]): DishDraft[] {
  return rows
    .map((r) => ({
      dish: r.dish.trim(),
      note: r.note.trim(),
      allergens: normalizeMealAllergens(r.allergens),
    }))
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
    rows.push(createDishDraft(e.dish, e.note ?? "", e.allergens));
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

  // The page is reachable with meals:read, but writing a day requires
  // meals:manage (the backend PUT/DELETE routes 403 otherwise). Readers get a
  // view-only plan: no edit, copy, paste, clear, add/remove, "Vorwoche
  // übernehmen", or save controls — so they never hit a guaranteed 403. The
  // exports need meals:orders, a child's registration changed by
  // the school meals:register (#3873). Mirror the nav's admin-OR-permission
  // gate.
  const canReadPlan = isAdmin(session) || hasPermission(session, "meals:read");
  const canEdit = isAdmin(session) || hasPermission(session, "meals:manage");
  const mealPlanEnabled = useMealPlanEnabled();
  const mealRegistrationEnabled = useMealRegistrationEnabled();
  // Die Bestellung der Woche braucht nur die Zahlen (#3873).
  const canReadOrders =
    status === "authenticated" &&
    mealRegistrationEnabled &&
    (isAdmin(session) || hasPermission(session, "meals:read"));
  const canReadParticipants =
    status === "authenticated" &&
    mealRegistrationEnabled &&
    (isAdmin(session) || hasPermission(session, "meals:orders"));
  const canRegister =
    status === "authenticated" &&
    mealRegistrationEnabled &&
    (isAdmin(session) || hasPermission(session, "meals:register"));

  // weekOffset: number of weeks from the current week (0 = this week, can go
  // negative for past weeks or beyond +1 for planning ahead).
  const [weekOffset, setWeekOffset] = useState(0);
  const [orderRefreshVersion, setOrderRefreshVersion] = useState(0);
  const [loading, setLoading] = useState(true);
  // True once the first week has loaded. Subsequent week switches keep the
  // current grid on screen (dimmed) instead of flashing the skeleton.
  const [hasLoaded, setHasLoaded] = useState(false);
  // True when the displayed week failed to load. The grid is then NOT shown:
  // editing/saving against an unknown persisted state could overwrite real
  // meals based on a failed load, so we surface an error + retry instead.
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const hasLoadError = Boolean(loadError);
  const [saving, setSaving] = useState(false);
  const saveErrors = useApiFormError();
  const { clear: clearSaveError } = saveErrors;
  const { show: showActionError } = useApiErrorDisplay();
  // „Wiederholen“ läuft mit dem aktuellen Stand der Seite (Woche, Entwürfe).
  const reloadRef = useRef<() => void>(() => undefined);
  const retrySaveRef = useRef<() => void>(() => undefined);
  const retryCopyPrevRef = useRef<() => void>(() => undefined);
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
  // Legende unter dem Plan: nur die Buchstaben, die diese Woche vorkommen.
  const weekAllergens = usedMealAllergens(
    weekDates.flatMap((date) => drafts[date] ?? []),
  );

  // Latest-wins guard: every load() captures a token; a slower earlier load
  // (e.g. a previous week on a slow connection) bails instead of overwriting
  // the newer week's drafts/originals after it resolves. A counter rather than
  // a per-effect cancelled flag because load() is also called by the retry
  // button and after save, not only from the week-switch effect.
  const loadTokenRef = useRef(0);

  const load = useCallback(async () => {
    const token = ++loadTokenRef.current;
    setLoading(true);
    clearLoadError();
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
      void showLoadError(err, {
        object: "die Woche im Speiseplan",
        retry: () => reloadRef.current(),
      });
    } finally {
      // Only the most recent load owns the shared loading/loaded flags.
      if (loadTokenRef.current === token) {
        setLoading(false);
        setHasLoaded(true);
      }
    }
  }, [mondayISO, weekDates, showLoadError, clearLoadError]);
  useLayoutEffect(() => {
    reloadRef.current = () => void load();
  });

  useEffect(() => {
    if (status !== "authenticated" || !canReadPlan || !mealPlanEnabled) return;
    void load();
  }, [load, status, canReadPlan, mealPlanEnabled]);

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
    field: "dish" | "note",
    value: string,
  ) => {
    setDrafts((prev) => {
      const rows = [...(prev[date] ?? [])];
      rows[idx] = { ...rows[idx]!, [field]: value };
      return { ...prev, [date]: rows };
    });
  };

  const updateAllergens = (date: string, idx: number, codes: string[]) => {
    setDrafts((prev) => {
      const rows = [...(prev[date] ?? [])];
      rows[idx] = { ...rows[idx]!, allergens: normalizeMealAllergens(codes) };
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
    if (
      row &&
      (row.dish.trim() !== "" ||
        row.note.trim() !== "" ||
        row.allergens.length > 0)
    ) {
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
    toast.success(`Die Gerichte vom ${weekdayLabel(date)} sind kopiert.`);
  };

  const pasteDay = (date: string) => {
    if (!clipboard) return;
    const rows = clipboard.length
      ? clipboard.map((row) =>
          createDishDraft(row.dish, row.note, row.allergens),
        )
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
        rows.push(createDishDraft(e.dish, e.note ?? "", e.allergens));
        byDate.set(e.date, rows);
      }
      setDrafts((prev) => {
        const next = { ...prev };
        prevDates.forEach((pd, i) => {
          const target = weekDates[i]!;
          const rows = byDate.get(pd);
          next[target] = rows?.length
            ? rows.map((row) =>
                createDishDraft(row.dish, row.note, row.allergens),
              )
            : [createDishDraft()];
        });
        return next;
      });
      toast.success(
        "Die Gerichte der Vorwoche sind übernommen. Bitte speichern Sie den Plan.",
      );
    } catch (err) {
      logger.error("meal_plan_copy_prev_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showActionError(err, {
        object: "die Vorwoche im Speiseplan",
        retry: () => retryCopyPrevRef.current(),
      });
    } finally {
      setCopyingPrev(false);
    }
  }, [mondayISO, weekDates, toast, showActionError]);
  useLayoutEffect(() => {
    retryCopyPrevRef.current = () => void doCopyPreviousWeek();
  });

  const requestCopyPreviousWeek = () => {
    if (weekDates.some((d) => dayHasContent(drafts[d]))) {
      setConfirmCopyPrev(true);
    } else {
      void doCopyPreviousWeek();
    }
  };

  // --- Save / discard ---------------------------------------------------
  const discard = () => {
    clearSaveError();
    setDrafts((prev) => {
      const next = { ...prev };
      for (const date of weekDates) {
        const o = originals[date] ?? [];
        next[date] = o.length
          ? o.map((row) => createDishDraft(row.dish, row.note, row.allergens))
          : [createDishDraft()];
      }
      return next;
    });
  };

  const handleSave = async () => {
    setSaving(true);
    clearSaveError();
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
            allergens: d.allergens,
          })),
        );
        persisted[date] = draft;
      }
      toast.success("Der Speiseplan ist gespeichert.");
      await load();
    } catch (err) {
      logger.error("meal_plan_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Commit the days that did persist; the still-unsaved days stay dirty.
      if (Object.keys(persisted).length > 0) {
        setOriginals((prev) => ({ ...prev, ...persisted }));
      }
      // Der Fehler steht über dem Wochenplan (Bauart 2 Regel 5), nicht als
      // Toast: die noch ungespeicherten Tage bleiben sichtbar markiert.
      void saveErrors.show(err, {
        object: "die Woche im Speiseplan",
        retry: () => retrySaveRef.current(),
      });
    } finally {
      setSaving(false);
    }
  };
  useLayoutEffect(() => {
    retrySaveRef.current = () => void handleSave();
  });

  // Dasselbe Wochenetikett wie Dienstplan, Zeiterfassung und Betreuungsplan
  // ("KW 41 · 05.10.–09.10.2026"), hier für die fünf Essenstage.
  const weekLabel = formatWeekLabel(
    parseISODate(weekDates[0]!),
    parseISODate(weekDates[4]!),
  );
  const isCurrentWeek = weekOffset === 0;
  // Kein Wochenwechsel, solange eine Woche lädt, gespeichert oder aus der
  // Vorwoche übernommen wird (siehe attemptWeekChange).
  const weekSwitchLocked = loading || saving || copyingPrev;

  // Statuszeile unter dem Titel: wie viele der fünf Wochentage schon ein
  // Gericht haben. Kalenderwoche und Zeitraum stehen im Bedienband direkt
  // darunter und werden nicht wiederholt (TENANT-PAGE-SPEC Regel 3). Die
  // Zahl kommt aus den bereits gespeicherten Tagen der geladenen Woche.
  const plannedDays = weekDates.filter(
    (date) => (originals[date]?.length ?? 0) > 0,
  ).length;
  const initialLoading = canReadPlan && loading && !hasLoaded;
  // Die Zahl der geplanten Tage gibt es nur für eine geladene Woche: nach
  // einem Ladefehler stünde sonst "0 von 5 Tagen geplant" da (#2517).
  const weekLoaded = weekDates.every((date) => date in originals);
  const statusLine = weekLoaded
    ? `${plannedDays} von ${weekDates.length} Tagen geplant`
    : null;

  return (
    <TenantPage
      title="Speiseplan"
      stats={statusLine}
      statsLoading={initialLoading}
      loading={initialLoading}
      error={loadError}
      // Bauart 3 Regel 1: der Wochenwechsel steuert Wochenplan und
      // Bestellung und sitzt darum im Bedienband der Kopfkarte.
      searchSlot={
        canReadPlan ? (
          <PlanningContextBar
            withoutContextRow
            dateLabel={weekLabel}
            onPrevious={
              weekSwitchLocked
                ? undefined
                : () => attemptWeekChange(weekOffset - 1)
            }
            onNext={
              weekSwitchLocked
                ? undefined
                : () => attemptWeekChange(weekOffset + 1)
            }
            previousLabel="Vorherige Woche"
            nextLabel="Nächste Woche"
            onToday={
              isCurrentWeek || weekSwitchLocked
                ? undefined
                : () => attemptWeekChange(0)
            }
            todayLabel="Diese Woche"
          />
        ) : undefined
      }
      overlays={
        <>
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
            confirmVariant="danger"
          >
            <p className="text-sm text-gray-600">
              Sie haben Änderungen in dieser Woche, die noch nicht gespeichert
              sind. Beim Wochenwechsel gehen sie verloren.
            </p>
          </ConfirmationModal>

          <ConfirmationModal
            isOpen={pendingHref !== null}
            onClose={cancelNavigation}
            onConfirm={confirmNavigation}
            title="Ungespeicherte Änderungen"
            confirmText="Verwerfen & verlassen"
            cancelText="Hierbleiben"
            confirmVariant="danger"
          >
            <p className="text-sm text-gray-600">
              Sie haben Änderungen, die noch nicht gespeichert sind. Beim
              Verlassen der Seite gehen sie verloren.
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
              Die aktuelle Woche wird mit dem Plan der Vorwoche überschrieben.
              Die Änderung wird erst mit „Speichern“ übernommen.
            </p>
          </ConfirmationModal>
        </>
      }
    >
      <FormErrorAlert message={saveErrors.error} />

      {/* Eine Karte für den Wochenplan: Titel, Erklärung und „Vorwoche
          übernehmen“ im Kopf, die fünf Tage randlos unter einer Trennlinie.
          overflow visible: die Allergen-Auswahl eines Gerichts darf über die
          Unterkante der Karte hinausragen. */}
      <SectionCard
        className={canReadPlan ? undefined : "hidden"}
        overflow="visible"
        title="Wochenplan"
        description="Pro Tag ein oder mehrere Gerichte mit Allergenen und optionalem Hinweis. Eltern sehen den Plan im Elternportal."
        actions={
          canEdit ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={requestCopyPreviousWeek}
              disabled={loading || saving || copyingPrev || hasLoadError}
              isLoading={copyingPrev}
              loadingText="Übernehmen…"
            >
              <Copy className="mr-2 h-4 w-4" />
              Vorwoche übernehmen
            </Button>
          ) : undefined
        }
      >
        <div
          className={`compact:-mx-4 compact:-mb-4 -mx-5 -mb-5 border-t border-gray-200 transition-opacity duration-200 max-sm:-mx-4 max-sm:-mb-4 ${
            loading ? "opacity-50" : "opacity-100"
          }`}
        >
          <div className="grid grid-cols-1 divide-y divide-gray-200 md:grid-cols-5 md:divide-y-0">
            {weekDates.map((date, dayIndex) => {
              const rows = drafts[date] ?? [];
              const isToday = date === today;
              const hasContent = dayHasContent(rows);
              return (
                <div
                  key={date}
                  data-meal-day-column
                  className={`flex flex-col border-gray-200 md:border-r md:last:border-r-transparent ${isToday ? "bg-moto-green/[0.04]" : ""}`}
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
                        <StatusBadge label="Heute" tone="green" compact />
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
                        {canEditNow ? (
                          <MealAllergenSelect
                            value={row.allergens}
                            // Donnerstag und Freitag öffnen nach links, damit
                            // das Menü im Fenster bleibt.
                            menuAlign={dayIndex >= 3 ? "end" : "start"}
                            onChange={(codes) =>
                              updateAllergens(date, idx, codes)
                            }
                            className="mt-2"
                          />
                        ) : (
                          <MealAllergenMarks
                            allergens={row.allergens}
                            className="mt-2"
                          />
                        )}
                        {canEdit &&
                          (rows.length > 1 ||
                            row.dish.trim() !== "" ||
                            row.note.trim() !== "" ||
                            row.allergens.length > 0) && (
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
          {weekAllergens.length > 0 ? (
            <MealAllergenLegend
              allergens={weekAllergens}
              showSource={false}
              className="border-t border-gray-200 px-4 py-3"
            />
          ) : null}
        </div>
      </SectionCard>

      {canReadOrders ? (
        <MealOrderWeek
          mondayISO={mondayISO}
          canExportMonth={canReadParticipants}
          refreshVersion={orderRefreshVersion}
        />
      ) : null}

      {/* Der Caterer der Schule (ADR 0077): nur wer den Speiseplan
          verwaltet, lädt ihn ein oder beendet die Belieferung. Er steht
          neben der Bestellung, die er beliefert. */}
      {canEdit && mealRegistrationEnabled === true ? (
        <MealCaterer canManageSupply={isAdmin(session)} />
      ) : null}

      {/* Die Tagesliste ist die längste Fläche und steht darum zuletzt:
          sie wächst bis zur Unterkante des Bildschirms. */}
      {canReadParticipants || canRegister ? (
        <MealParticipantList
          canRegister={canRegister}
          canExport={canReadParticipants}
          onRegistrationChanged={() =>
            setOrderRefreshVersion((value) => value + 1)
          }
        />
      ) : null}

      {/* Sticky save bar — only while there are unsaved changes. */}
      {canEdit && isDirty && !loading && !hasLoadError && (
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
  );
}
