"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  AlarmClock,
  BookOpen,
  Check,
  Clock3,
  Clock4,
  Download,
  FileSpreadsheet,
  Palette,
  Printer,
  Rows3,
  Utensils,
  type LucideIcon,
} from "lucide-react";
import { PlanningDisabledState } from "~/components/planning/planning-disabled-state";
import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DatePicker } from "~/components/ui/date-picker";
import { Loading } from "~/components/ui/loading";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { DesktopFilters } from "~/components/ui/page-header/DesktopFilters";
import { ActiveFilterChips } from "~/components/ui/page-header/ActiveFilterChips";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  berlinTodayISO,
  isValidISODate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  exportSlotList,
  fetchSlotListOptions,
  fetchSlotListPreview,
  groupByOptionsFor,
  SLOT_LIST_KIND_LABELS,
  SLOT_LIST_GROUP_BY_LABELS,
  SLOT_LIST_SOURCE_LABELS,
  type SlotListFormat,
  type SlotListGroupBy,
  type SlotListKind,
  type SlotListOptionsResult,
  type SlotListPickupCohort,
  type SlotListResult,
  type SlotListRow,
  type SlotListSource,
  type SlotListTarget,
} from "~/lib/slot-lists-api";

const logger = createLogger({ component: "SlotListsPage" });

interface SelectionOption {
  id: "slots" | SlotListPickupCohort | SlotListKind;
  target: SlotListTarget;
  pickupCohort?: SlotListPickupCohort;
  listKind?: SlotListKind;
  label: string;
  description: string;
  icon: LucideIcon;
}

// One short, single-line description each so every card is the same height.
const SELECTION_OPTIONS: SelectionOption[] = [
  {
    id: "edge_hours",
    target: "slots",
    listKind: "edge_hours",
    label: SLOT_LIST_KIND_LABELS.edge_hours,
    description: "Über Listenart zugeordnet",
    icon: AlarmClock,
  },
  {
    id: "learning_time",
    target: "slots",
    listKind: "learning_time",
    label: SLOT_LIST_KIND_LABELS.learning_time,
    description: "Über Listenart zugeordnet",
    icon: BookOpen,
  },
  {
    id: "activity",
    target: "slots",
    listKind: "activity",
    label: SLOT_LIST_KIND_LABELS.activity,
    description: "Über Listenart zugeordnet",
    icon: Palette,
  },
  {
    id: "mensa",
    target: "slots",
    listKind: "mensa",
    label: SLOT_LIST_KIND_LABELS.mensa,
    description: "Über Listenart zugeordnet",
    icon: Utensils,
  },
  {
    id: "short_day",
    target: "pickup_cohort",
    pickupCohort: "short_day",
    label: "Ganztag bis 14:30",
    description: "Abholung bis 14:30 Uhr",
    icon: Clock3,
  },
  {
    id: "long_day",
    target: "pickup_cohort",
    pickupCohort: "long_day",
    label: "Ganztag bis 16:00",
    description: "Abholung nach 14:30 Uhr",
    icon: Clock4,
  },
  {
    id: "slots",
    target: "slots",
    label: "Freie Angebotsauswahl",
    description: "Angebote des Tages auswählen",
    icon: Rows3,
  },
];

const SOURCES: SlotListSource[] = ["planned", "actual", "reconciliation"];
const CANCELLED_SLOT_STATUS = "cancelled";
const PREVIEW_PAGE_SIZE = 50;

const SOURCE_TO_URL: Record<SlotListSource, string> = {
  planned: "plan",
  actual: "ist",
  reconciliation: "abgleich",
};

const URL_TO_SOURCE: Record<string, SlotListSource> = {
  plan: "planned",
  ist: "actual",
  abgleich: "reconciliation",
};

const GROUP_BY_TO_URL: Record<Exclude<SlotListGroupBy, "">, string> = {
  slot: "angebot",
  room: "raum",
  class: "klasse",
  pickup_time: "abholzeit",
};

const URL_TO_GROUP_BY: Record<string, SlotListGroupBy> = {
  angebot: "slot",
  raum: "room",
  klasse: "class",
  abholzeit: "pickup_time",
};

const SELECTION_TO_URL: Record<SelectionOption["id"], string> = {
  slots: "angebote",
  edge_hours: "randstunden",
  learning_time: "lernzeit",
  activity: "ag-angebote",
  mensa: "mensa",
  short_day: "ganztag-1430",
  long_day: "ganztag-1600",
};

const URL_TO_SELECTION: Record<string, SelectionOption["id"]> = {
  angebote: "slots",
  randstunden: "edge_hours",
  lernzeit: "learning_time",
  "ag-angebote": "activity",
  mensa: "mensa",
  "ganztag-1430": "short_day",
  "ganztag-1600": "long_day",
};

interface QueryParamsLike {
  get(name: string): string | null;
  getAll(name: string): string[];
}

interface InitialListState {
  target: SlotListTarget;
  pickupCohort: SlotListPickupCohort;
  listKind: SlotListKind | "";
  source: SlotListSource;
  groupBy: SlotListGroupBy;
  dateISO: string;
  selectedSlotIds: string[] | null;
  selectedGroupIds: string[] | null;
  selectedClasses: string[] | null;
  // True when an id filter param (gruppen/angebote) is present in the URL but
  // contains no usable id — a corrupted or hand-edited link. The page refuses
  // it instead of silently widening the list (see isCorruptIdParam).
  filterLinkInvalid: boolean;
}

function statusColor(row: SlotListRow, source: SlotListSource): string {
  if (source === "planned") return LOCATION_COLORS.OTHER_ROOM;
  if (source === "actual") return LOCATION_COLORS.GROUP_ROOM;
  if (row.unplanned) return LOCATION_COLORS.SCHOOLYARD;
  if (row.present) return LOCATION_COLORS.GROUP_ROOM;
  // Justified (registered) absence — purple, visibly apart from an
  // unexplained "Fehlt" (red).
  if (row.excused) return LOCATION_COLORS.EXCUSED;
  if (row.planned) return LOCATION_COLORS.HOME;
  return LOCATION_COLORS.OTHER_ROOM;
}

// CounterChips surfaces result.counters so users can verify the summary the
// help guide points them to (geplant / anwesend / fehlt / abgemeldet /
// ungeplant) without counting table rows. The backend zeroes the counters
// that are meaningless for the source (plan and Ist carry only their own
// number; the merge counters exist only in the Abgleich).
function CounterChips({
  counters,
  source,
}: Readonly<{
  counters: SlotListResult["counters"];
  source: SlotListSource;
}>) {
  const chips: Array<{ label: string; value: number; color: string }> =
    source === "planned"
      ? [
          {
            label: "Geplant",
            value: counters.planned,
            color: LOCATION_COLORS.OTHER_ROOM,
          },
        ]
      : source === "actual"
        ? [
            {
              label: "Anwesend",
              value: counters.present,
              color: LOCATION_COLORS.GROUP_ROOM,
            },
          ]
        : [
            {
              label: "Geplant",
              value: counters.planned,
              color: LOCATION_COLORS.OTHER_ROOM,
            },
            {
              label: "Anwesend",
              value: counters.present,
              color: LOCATION_COLORS.GROUP_ROOM,
            },
            {
              label: "Fehlt",
              value: counters.missing,
              color: LOCATION_COLORS.HOME,
            },
            {
              label: "Abgemeldet",
              value: counters.excused,
              color: LOCATION_COLORS.EXCUSED,
            },
            {
              label: "Ungeplant",
              value: counters.unplanned,
              color: LOCATION_COLORS.SCHOOLYARD,
            },
          ];
  return (
    <div className="flex flex-wrap items-center gap-2" aria-label="Zähler">
      {chips.map((chip) => (
        <span
          key={chip.label}
          className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium text-gray-600"
        >
          <span
            aria-hidden
            className="inline-block h-1.5 w-1.5 rounded-full"
            style={{ backgroundColor: chip.color }}
          />
          {chip.label}
          <span className="font-semibold text-gray-900 tabular-nums">
            {chip.value}
          </span>
        </span>
      ))}
    </div>
  );
}

function StatusBadge({
  row,
  source,
}: Readonly<{ row: SlotListRow; source: SlotListSource }>) {
  const color = statusColor(row, source);
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-xs font-medium">
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: color }}
      />
      <span style={{ color }}>{row.status_label}</span>
    </span>
  );
}

// nextSelection collapses a multi-select to null ("all") when the chosen set is
// empty or covers every current option; otherwise it keeps the subset.
//
// It compares MEMBERSHIP against the actual option values, not just the count.
// A bookmarked `gruppen`/`klassen` filter can carry a value that is absent from
// today's options (a stale ID, a group with no rows today, a duplicated URL
// value). Such a value lingers in the dropdown's selection while not being a
// current option, so a plain `values.length === total` compare could report a
// partial selection as "complete" and return null — silently dropping a
// confidential group/class restriction and broadening the named-student
// preview/export to the whole cohort. Deduplicating and dropping unknown values
// keeps the restriction accurate and prunes the stale selection (#1565 review).
function nextSelection<T>(values: T[], options: T[]): T[] | null {
  const optionSet = new Set(options);
  const chosen = [...new Set(values)].filter((value) => optionSet.has(value));
  if (chosen.length === 0) return null; // nothing valid chosen → no restriction
  if (chosen.length === optionSet.size) return null; // every option → no restriction
  return chosen;
}

function parseIdList(
  value: string | null,
  emptyToken: string | null = null,
): string[] | null {
  if (!value) return null;
  if (emptyToken && value === emptyToken) return [];
  const ids = value
    .split(",")
    .map((part) => part.trim())
    .filter((id) => /^\d+$/.test(id) && id !== "0");
  return ids.length > 0 ? ids : null;
}

// isCorruptIdParam reports whether an id filter param is present but resolves to
// no usable id — e.g. a copied URL with `gruppen=-1` or `angebote=0`.
// parseIdList returns null for BOTH "param absent" and "param present but every
// token invalid", and null reads as "no restriction" everywhere downstream: for
// a group filter that would silently widen a confidential single-group export to
// the whole cohort (the backend treats an empty group set as no restriction),
// and for a manual slot filter it would fall back to every active offering.
// Detecting the corrupted-link case lets the page reject it rather than export a
// broadened, named-student list (#1565 review pass 3). A partially-invalid param
// (e.g. `gruppen=5,-1`) keeps its valid subset and is NOT treated as corrupt.
function isCorruptIdParam(
  value: string | null,
  emptyToken: string | null = null,
): boolean {
  if (value === null) return false; // absent → legitimately unrestricted
  if (emptyToken && value === emptyToken) return false; // explicit empty selection
  // A present-but-empty value (`gruppen=`, `angebote=`) is NOT an absent param:
  // parseIdList collapses it to null ("no restriction"), which for a confidential
  // group/slot filter would silently broaden the named-student list to the whole
  // cohort. Distinguish "" from null and treat it as a corrupt link (#1565 review
  // pass 4).
  return parseIdList(value, emptyToken) === null; // present but nothing valid
}

// Class names are free-form backend text and may contain commas (e.g.
// "1a, bilingual"), so they cannot share a comma-delimited param with other
// classes — a single class would round-trip as two nonexistent filters, which
// the reconciliation effect would then prune away and broaden the list. They
// travel as repeated `klassen` query params instead, one value each (#1565
// review pass 2).
function parseClassList(values: string[]): string[] | null {
  const cleaned = values.map((part) => part.trim()).filter(Boolean);
  return cleaned.length > 0 ? cleaned : null;
}

// isCorruptClassParam mirrors isCorruptIdParam for the repeated `klassen` params:
// a present-but-blank value (`klassen=`) cleans to nothing, so parseClassList
// collapses it to null ("no restriction") and a confidential class restriction
// would silently broaden to every class. A partially-blank set
// (`klassen=1a&klassen=`) keeps its valid subset and is NOT corrupt (#1565 review
// pass 4).
function isCorruptClassParam(values: string[]): boolean {
  if (values.length === 0) return false; // absent → legitimately unrestricted
  return parseClassList(values) === null; // present but nothing valid
}

// Ordered content equality for the URL-synced selection arrays. The
// serialize→parse round-trip preserves element order, so a positional compare
// is enough to tell an unchanged selection from a real one and avoid installing
// a fresh-but-equal reference (#1565 review).
function sameStringList(a: string[] | null, b: string[] | null): boolean {
  if (a === b) return true;
  if (a === null || b === null) return false;
  if (a.length !== b.length) return false;
  return a.every((value, index) => value === b[index]);
}

function defaultSelectionOption(): SelectionOption {
  const option = SELECTION_OPTIONS[0];
  if (!option) {
    throw new Error("Tageslisten-Auswahl ist nicht konfiguriert.");
  }
  return option;
}

function parseInitialListState(
  searchParams: QueryParamsLike,
): InitialListState {
  const selectionId =
    URL_TO_SELECTION[searchParams.get("quelle") ?? ""] ?? "slots";
  const option =
    SELECTION_OPTIONS.find((item) => item.id === selectionId) ??
    defaultSelectionOption();
  const target = option.target;
  const pickupCohort = option.pickupCohort ?? "short_day";
  const listKind = option.listKind ?? "";
  const source = URL_TO_SOURCE[searchParams.get("basis") ?? ""] ?? "planned";
  const rawGroupBy =
    URL_TO_GROUP_BY[searchParams.get("gruppieren") ?? ""] ?? "";

  // isValidISODate (round-trip), not a bare shape check: "2026-02-31" would
  // pass the regex, roll into March in the picker, and leave the backend
  // rejecting the unchanged URL date. Default is the SCHOOL's calendar day
  // (Berlin), because todayISO() would use the browser's timezone and open
  // the wrong day's list around midnight.
  const rawDate = searchParams.get("datum");
  const dateISO =
    rawDate && isValidISODate(rawDate) ? rawDate : berlinTodayISO();

  return {
    target,
    pickupCohort,
    listKind,
    source,
    groupBy: groupByOptionsFor(target).includes(rawGroupBy) ? rawGroupBy : "",
    dateISO,
    selectedSlotIds: parseIdList(searchParams.get("angebote"), "keine"),
    selectedGroupIds: parseIdList(searchParams.get("gruppen")),
    selectedClasses: parseClassList(searchParams.getAll("klassen")),
    filterLinkInvalid:
      isCorruptIdParam(searchParams.get("gruppen")) ||
      isCorruptIdParam(searchParams.get("angebote"), "keine") ||
      isCorruptClassParam(searchParams.getAll("klassen")),
  };
}

// filterLinkInvalid is a parse-time diagnostic only, never serialized back into
// the URL, so serialize operates on the state minus that field.
function serializeListState(
  state: Omit<InitialListState, "filterLinkInvalid">,
): string {
  const params = new URLSearchParams();
  params.set("datum", state.dateISO);

  const selectionId =
    state.target === "pickup_cohort"
      ? state.pickupCohort
      : state.listKind || "slots";
  params.set("quelle", SELECTION_TO_URL[selectionId]);
  params.set("basis", SOURCE_TO_URL[state.source]);

  if (state.groupBy) {
    params.set("gruppieren", GROUP_BY_TO_URL[state.groupBy]);
  }
  if (
    state.target === "slots" &&
    !state.listKind &&
    state.selectedSlotIds !== null
  ) {
    params.set(
      "angebote",
      state.selectedSlotIds.length === 0
        ? "keine"
        : state.selectedSlotIds.join(","),
    );
  }
  if (state.selectedGroupIds !== null) {
    params.set("gruppen", state.selectedGroupIds.join(","));
  }
  if (state.selectedClasses !== null) {
    // One repeated param per class — class names are free text and may contain
    // commas, so a comma-joined value could not round-trip (#1565 review pass 2).
    for (const schoolClass of state.selectedClasses) {
      params.append("klassen", schoolClass);
    }
  }

  return params.toString();
}

export default function SlotListsPage() {
  const { status: authStatus } = useSession({ required: true });
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  // Route guard: Tageslisten are part of the timetable feature, so an explicit
  // timetable.enabled=false hides the page just like the Betreuungsplan/Vertretung
  // guards and the sidebar (#1565 review). getSettingValue returns undefined when
  // the user cannot read settings, so the page renders normally by default.
  const { data: settingsSchema, isLoading: settingsSchemaLoading } =
    useSettingsSchema(authStatus === "authenticated", {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    });
  const timetableDisabled =
    getSettingValue(settingsSchema, "timetable.enabled") === false;
  const initialState = useMemo(
    () => parseInitialListState(searchParams),
    [searchParams],
  );
  const [target, setTarget] = useState<SlotListTarget>(initialState.target);
  const [pickupCohort, setPickupCohort] = useState<SlotListPickupCohort>(
    initialState.pickupCohort,
  );
  const [listKind, setListKind] = useState<SlotListKind | "">(
    initialState.listKind,
  );
  const [source, setSource] = useState<SlotListSource>(initialState.source);
  const [groupBy, setGroupBy] = useState<SlotListGroupBy>(initialState.groupBy);
  const [dateISO, setDateISO] = useState<string>(initialState.dateISO);
  // null = no restriction on that dimension; otherwise an explicit subset.
  const [selectedSlotIds, setSelectedSlotIds] = useState<string[] | null>(
    initialState.selectedSlotIds,
  );
  const [selectedGroupIds, setSelectedGroupIds] = useState<string[] | null>(
    initialState.selectedGroupIds,
  );
  const [selectedClasses, setSelectedClasses] = useState<string[] | null>(
    initialState.selectedClasses,
  );
  // A corrupted id filter in the URL (see isCorruptIdParam) blocks the preview
  // and exports instead of running a silently-widened query. It self-clears on
  // the next filter/date/source change: the bad param never round-trips back
  // into the URL because the parsed selection is already null.
  const [filterLinkInvalid, setFilterLinkInvalid] = useState<boolean>(
    initialState.filterLinkInvalid,
  );
  const [listOptions, setListOptions] = useState<SlotListOptionsResult | null>(
    null,
  );
  const [result, setResult] = useState<SlotListResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isOptionsLoading, setIsOptionsLoading] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Non-null once /options rejects. Without it a failed options request leaves
  // listOptions null forever, so activeInstanceIds stays null and
  // awaitingActiveSlots never clears — the preview gate below would then hold
  // isLoading true indefinitely (permanent spinner, exports disabled) even
  // though /preview itself could succeed (#1565 review pass 1).
  const [optionsError, setOptionsError] = useState<string | null>(null);

  const isPickupBased = target === "pickup_cohort";
  const isManualSlotSelection = target === "slots" && listKind === "";

  // The authoritative active (non-cancelled) instance set for the date, taken
  // from the options endpoint ONLY — never the preview result. The default "all
  // active" state (selectedSlotIds === null) sends these explicitly so a
  // cancelled-but-started slot — which keeps its ActiveGroupID and is otherwise
  // processed when instance_ids is omitted ("all") — is excluded from Ist and
  // Abgleich, matching what the UI shows as unselected. null while options load
  // (omit → backend "all") avoids a circular dependency with the preview result
  // that selectableSlotIds falls back to (#1565 review).
  const activeInstanceIds = useMemo<string[] | null>(
    () =>
      listOptions
        ? listOptions.slots
            .filter((slot) => slot.status !== CANCELLED_SLOT_STATUS)
            .map((slot) => slot.instance_id)
        : null,
    [listOptions],
  );

  const request = useMemo(
    () => ({
      date: dateISO,
      target,
      ...(isPickupBased ? { pickup_cohort: pickupCohort } : {}),
      ...(!isPickupBased && listKind ? { list_kind: listKind } : {}),
      source,
      ...(groupBy ? { group_by: groupBy } : {}),
      // Explicit user subset wins (free selection only); otherwise send the
      // active set so "all active" never collapses to the backend's
      // omitted-means-all path. This applies to classified lists too: the
      // backend intersects list_kind with instance_ids, so a slot that was
      // started and then cancelled — which keeps its ActiveGroupID and would
      // otherwise leak the visits recorded during its brief run into Ist and
      // Abgleich under the omitted path — is excluded, matching what /options
      // counts and what the card reports as assigned slots (#1565 review).
      // A bookmarked explicit selection is intersected with the active set
      // before it reaches the backend: a since-cancelled or deleted slot ID
      // from a saved URL would otherwise be honored as an explicit request and
      // leak its retained cancelled-slot attendance — including in the single
      // render before the pruning effect rewrites state (#1565 review pass 1).
      // An all-stale selection intersects to [] ("none"), never omitted/"all".
      ...(target === "slots"
        ? isManualSlotSelection && selectedSlotIds !== null
          ? activeInstanceIds !== null
            ? {
                instance_ids: selectedSlotIds.filter((id) =>
                  activeInstanceIds.includes(id),
                ),
              }
            : {}
          : activeInstanceIds !== null
            ? { instance_ids: activeInstanceIds }
            : {}
        : {}),
      ...(selectedGroupIds ? { group_ids: selectedGroupIds } : {}),
      ...(selectedClasses ? { classes: selectedClasses } : {}),
    }),
    [
      dateISO,
      target,
      pickupCohort,
      isPickupBased,
      isManualSlotSelection,
      listKind,
      source,
      groupBy,
      selectedSlotIds,
      activeInstanceIds,
      selectedGroupIds,
      selectedClasses,
    ],
  );

  // For slot lists the active (non-cancelled) instance set is authoritative and
  // comes from /options, never the preview. Until it loads, activeInstanceIds is
  // null and the request omits instance_ids — which the backend reads as "all
  // slots", pulling in a cancelled-but-started slot whose brief-run attendance we
  // deliberately exclude. Firing the preview then would render, and enable
  // print/download on, rows the UI marks as cancelled-excluded (#1565 review
  // pass 1). Hold the preview until the active set lands — including for an
  // explicit bookmarked selection: a saved slot ID cannot be validated (nor a
  // since-cancelled one intersected out of the request) until /options resolves,
  // so a manual subset must wait too rather than fire a preview off unvalidated
  // IDs (#1565 review pass 1). Pickup lists do not use instance_ids at all.
  const awaitingActiveSlots = target === "slots" && activeInstanceIds === null;

  const replaceListUrl = useCallback(
    (next: Partial<InitialListState>) => {
      const query = serializeListState({
        target,
        pickupCohort,
        listKind,
        source,
        groupBy,
        dateISO,
        selectedSlotIds,
        selectedGroupIds,
        selectedClasses,
        ...next,
      });
      router.replace(`/lists?${query}`);
    },
    [
      target,
      pickupCohort,
      source,
      groupBy,
      dateISO,
      listKind,
      selectedSlotIds,
      selectedGroupIds,
      selectedClasses,
      router,
    ],
  );

  useEffect(() => {
    const next = parseInitialListState(searchParams);
    setTarget(next.target);
    setPickupCohort(next.pickupCohort);
    setListKind(next.listKind);
    setSource(next.source);
    setGroupBy(next.groupBy);
    setDateISO(next.dateISO);
    // Keep the previous reference when the URL round-trip yields an equal
    // selection. A slot/group/class change first updates local state (firing
    // the preview), then router.replace re-parses searchParams into fresh
    // arrays here; installing them would recompute `request` and fire a second,
    // redundant BuildList for the same selection (#1565 review).
    setSelectedSlotIds((prev) =>
      sameStringList(prev, next.selectedSlotIds) ? prev : next.selectedSlotIds,
    );
    setSelectedGroupIds((prev) =>
      sameStringList(prev, next.selectedGroupIds)
        ? prev
        : next.selectedGroupIds,
    );
    setSelectedClasses((prev) =>
      sameStringList(prev, next.selectedClasses) ? prev : next.selectedClasses,
    );
    setFilterLinkInvalid(next.filterLinkInvalid);
  }, [searchParams]);

  const pickupOptionByCohort = useMemo(() => {
    const map = new Map<
      SlotListPickupCohort,
      SlotListOptionsResult["pickup_cohorts"][number]
    >();
    for (const option of listOptions?.pickup_cohorts ?? []) {
      map.set(option.cohort, option);
    }
    return map;
  }, [listOptions]);
  const optionSlots = listOptions?.slots;
  const resultSlots = result?.slots;
  const slotOptions = useMemo(
    () => optionSlots ?? resultSlots ?? [],
    [optionSlots, resultSlots],
  );
  const selectableSlotIds = useMemo(
    () =>
      slotOptions
        .filter((slot) => slot.status !== CANCELLED_SLOT_STATUS)
        .map((slot) => slot.instance_id),
    [slotOptions],
  );
  const cancelledSlotCount = slotOptions.length - selectableSlotIds.length;
  const selectedSlotIdsForUI = useMemo(
    () => selectedSlotIds ?? selectableSlotIds,
    [selectedSlotIds, selectableSlotIds],
  );
  const selectedSlotIdSet = useMemo(
    () => new Set(selectedSlotIdsForUI),
    [selectedSlotIdsForUI],
  );
  const slotSummary =
    slotOptions.length === 0
      ? "Keine Angebote geplant"
      : selectableSlotIds.length === 0
        ? "Keine aktiven Angebote auswählbar"
        : selectedSlotIds === null
          ? "Alle aktiven Angebote ausgewählt"
          : `${selectedSlotIds.length} von ${selectableSlotIds.length} ausgewählt`;

  // Ganztag pickup cohorts cannot be reconstructed for a past date (the backend
  // refuses with ErrPickupCohortPastDate); slot lists stay available. Compared
  // against the school's Berlin day, matching the date-picker default above.
  const isPastDate = dateISO < berlinTodayISO();

  // Constrain the date picker to the range the active mode allows, so the user
  // can never pick a date the backend rejects with a 400 (#1565 review):
  // Ganztag needs today or later, an Abgleich needs today or earlier. With both
  // active the picker collapses to today, the only valid pickup reconciliation.
  const berlinToday = parseISODate(berlinTodayISO());
  const pickerMinDate = target === "pickup_cohort" ? berlinToday : undefined;
  const pickerMaxDate = source === "reconciliation" ? berlinToday : undefined;

  // A new list type or date means a different cohort → drop all filters.
  const resetFilters = useCallback(() => {
    setSelectedSlotIds(null);
    setSelectedGroupIds(null);
    setSelectedClasses(null);
  }, []);
  const pickSelection = useCallback(
    (option: SelectionOption) => {
      setTarget(option.target);
      const nextPickupCohort = option.pickupCohort ?? pickupCohort;
      const nextListKind = option.listKind ?? "";
      if (option.pickupCohort) setPickupCohort(option.pickupCohort);
      setListKind(nextListKind);
      // slot/room/pickup_time groupings are target-specific, so a switch can
      // invalidate the current choice — reset to a flat list.
      setGroupBy("");
      resetFilters();
      replaceListUrl({
        target: option.target,
        pickupCohort: nextPickupCohort,
        listKind: nextListKind,
        groupBy: "",
        selectedSlotIds: null,
        selectedGroupIds: null,
        selectedClasses: null,
      });
    },
    [pickupCohort, replaceListUrl, resetFilters],
  );
  const pickDate = useCallback(
    (date: Date | null) => {
      if (!date) return;
      const nextDate = toISODate(date);
      setDateISO(nextDate);
      resetFilters();
      replaceListUrl({
        dateISO: nextDate,
        selectedSlotIds: null,
        selectedGroupIds: null,
        selectedClasses: null,
      });
    },
    [replaceListUrl, resetFilters],
  );
  const toggleSlot = useCallback(
    (slotId: string) => {
      if (!selectableSlotIds.includes(slotId)) return;
      const current = selectedSlotIds ?? selectableSlotIds;
      const next = current.includes(slotId)
        ? current.filter((id) => id !== slotId)
        : [...current, slotId];
      const nextSelection =
        next.length === selectableSlotIds.length ? null : next;
      setSelectedSlotIds(nextSelection);
      replaceListUrl({ selectedSlotIds: nextSelection });
    },
    [replaceListUrl, selectedSlotIds, selectableSlotIds],
  );

  useEffect(() => {
    if (authStatus !== "authenticated" || timetableDisabled) return;
    let cancelled = false;
    // Drop the previous date's options immediately. They stay rendered and
    // selectable until this request resolves otherwise, and on a slow response
    // toggling a stale slot would write its (globally unique) ID into the new
    // date's URL/request — producing an empty preview with no checkbox selected
    // once the new options arrive. selectableSlotIds empties with them, so the
    // slot controls disable until the correct set loads.
    setListOptions(null);
    setOptionsError(null);
    setIsOptionsLoading(true);
    fetchSlotListOptions(dateISO)
      .then((options) => {
        if (!cancelled) setListOptions(options);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("slot_list_options_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setListOptions(null);
        // Record the failure so the preview gate releases instead of spinning
        // forever. Prefer the API client's resolved German sentence (#1565).
        setOptionsError(
          err instanceof Error && err.message
            ? err.message
            : "Die Slot-Optionen konnten nicht geladen werden.",
        );
      })
      .finally(() => {
        if (!cancelled) setIsOptionsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authStatus, dateISO, timetableDisabled]);

  useEffect(() => {
    if (authStatus !== "authenticated" || timetableDisabled) return;
    // A corrupted id filter in the URL must not run a silently-widened query.
    // Clear the result (which also disables the export buttons) and surface an
    // invalid-link message instead of previewing/exporting every group or slot
    // (#1565 review pass 3). Checked before the awaitingActiveSlots gate below.
    if (filterLinkInvalid) {
      setResult(null);
      setError(
        "Dieser Link enthält einen ungültigen Filter und wurde nicht angewendet. Bitte Filter zurücksetzen oder die Liste erneut öffnen.",
      );
      setIsLoading(false);
      return;
    }
    // Hold the preview (and, via the cleared result, print/download) until the
    // active slot set resolves — see awaitingActiveSlots. The effect re-fires
    // when activeInstanceIds lands, because it feeds `request`. Clearing the
    // result also drops the previous date's rows, which would otherwise stay
    // exportable during the options-loading window after a date change.
    if (awaitingActiveSlots) {
      // The authoritative active slot set never arrived because /options
      // failed. Don't hold the gate — surface the options error and stop
      // loading so the user isn't stuck on a permanent spinner (#1565 review
      // pass 1).
      if (optionsError) {
        setResult(null);
        setError(optionsError);
        setIsLoading(false);
        return;
      }
      setResult(null);
      setError(null);
      setIsLoading(true);
      return;
    }
    let cancelled = false;
    setIsLoading(true);
    setError(null);
    fetchSlotListPreview(request)
      .then((preview) => {
        if (!cancelled) setResult(preview);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("slot_list_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setResult(null);
        // Surface the backend's explanation (e.g. the past-date Ganztag refusal
        // ErrPickupCohortPastDate) instead of a generic failure — the API client
        // already resolves it to a German sentence.
        setError(
          err instanceof Error && err.message
            ? err.message
            : "Die Vorschau konnte nicht geladen werden.",
        );
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    authStatus,
    request,
    timetableDisabled,
    awaitingActiveSlots,
    optionsError,
    filterLinkInvalid,
  ]);

  // We deliberately do NOT reconcile stale group/class URL filters against the
  // preview. result.groups / result.classes are derived from the preview rows,
  // so they list only the groups/classes that actually have rows for this date +
  // source — NOT the school's complete set. A selected group/class absent from
  // that derived set is ambiguous: it may have been deleted, or it may still be
  // valid but simply have no rows today. There is no authoritative complete
  // option set to disambiguate (the /options endpoint returns slots and cohorts,
  // not groups/classes), so pruning a partial selection down to only its
  // surviving members would silently narrow a saved multi-group export whenever
  // one of the requested groups happens to be rowless — dropping a group the
  // user explicitly asked to disclose. Leaving the explicit filter untouched
  // keeps the requested scope exact; the preview simply renders no rows for a
  // rowless member (#1565 review pass 2).
  //
  // Stale MANUAL SLOT selections are still pruned below because their selectable
  // set comes from the authoritative /options endpoint (listOptions), which CAN
  // distinguish a deleted/cancelled slot from a valid one.

  // Prune stale manual slot selections the same way. Unlike groups/classes the
  // selectable set comes from the date options (listOptions), not the preview,
  // so gate on those having loaded: during the options-loading window
  // listOptions is null and selectableSlotIds is empty, and pruning then would
  // wrongly collapse a real selection. A bookmarked list can point at an
  // occurrence that was since deleted, replanned, cancelled, or moved to
  // another date; without this the stale ID stays an explicit restriction and
  // empties the preview while the summary still claims slots are selected and
  // no checkbox is checked (#1565 review). An explicit empty ("keine")
  // selection has nothing to prune, so it survives unchanged.
  useEffect(() => {
    if (!isManualSlotSelection || selectedSlotIds === null || !listOptions) {
      return;
    }
    const available = new Set(selectableSlotIds);
    const pruned = selectedSlotIds.filter((id) => available.has(id));
    if (pruned.length === selectedSlotIds.length) return;
    // null means "all active slots" everywhere downstream, so only collapse to
    // it when the survivors ARE the whole active set. When every selected slot
    // is stale (pruned empty), keep an explicit empty selection ([] → "keine")
    // instead — collapsing to null would silently broaden a saved single-
    // offering list to every offering on the date (#1565 review pass 1).
    const next =
      pruned.length > 0 && pruned.length === selectableSlotIds.length
        ? null
        : pruned;
    setSelectedSlotIds(next);
    replaceListUrl({ selectedSlotIds: next });
  }, [
    isManualSlotSelection,
    listOptions,
    selectedSlotIds,
    selectableSlotIds,
    replaceListUrl,
  ]);

  const handleExport = useCallback(
    async (format: SlotListFormat, mode: "download" | "print") => {
      // Never export while the authoritative active slot set is still loading —
      // the request would omit instance_ids and the backend would include the
      // cancelled-but-started slots the UI excludes (#1565 review pass 1). The
      // buttons are already disabled in this window; this is defense in depth.
      if (awaitingActiveSlots) return;
      // Never export a silently-widened list from a corrupted filter link. The
      // buttons are disabled while no result exists; this is defense in depth.
      if (filterLinkInvalid) return;
      setIsExporting(true);
      setError(null);
      try {
        await exportSlotList(request, format, mode);
      } catch (err: unknown) {
        logger.error("slot_list_export_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(
          "Die Liste konnte nicht exportiert werden. Bitte versuchen Sie es erneut.",
        );
      } finally {
        setIsExporting(false);
      }
    },
    [request, awaitingActiveSlots, filterLinkInvalid],
  );

  // Standard filter row, rendered via the shared DesktopFilters component.
  const filterConfigs: FilterConfig[] = useMemo(() => {
    const configs: FilterConfig[] = [
      {
        id: "groupBy",
        label: "Gruppieren",
        type: "dropdown",
        value: groupBy,
        onChange: (v) => {
          const nextGroupBy = v as SlotListGroupBy;
          setGroupBy(nextGroupBy);
          replaceListUrl({ groupBy: nextGroupBy });
        },
        options: groupByOptionsFor(target).map((option) => ({
          value: option,
          label:
            option === ""
              ? "Nicht gruppiert"
              : SLOT_LIST_GROUP_BY_LABELS[option],
        })),
      },
    ];

    if (!result) return configs;

    if (result.groups.length > 1) {
      const groupIds = result.groups.map((g) => g.id);
      const active = selectedGroupIds ?? groupIds;
      configs.push({
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        multiSelect: true,
        value: active,
        onChange: (v) => {
          const nextGroupIds = nextSelection(v as string[], groupIds);
          setSelectedGroupIds(nextGroupIds);
          replaceListUrl({ selectedGroupIds: nextGroupIds });
        },
        options: result.groups.map((g) => ({
          value: g.id,
          label: g.name,
        })),
      });
    }

    if (result.classes.length > 1) {
      const active = selectedClasses ?? result.classes;
      configs.push({
        id: "class",
        label: "Klasse",
        type: "dropdown",
        multiSelect: true,
        value: active,
        onChange: (v) => {
          const nextClasses = nextSelection(v as string[], result.classes);
          setSelectedClasses(nextClasses);
          replaceListUrl({ selectedClasses: nextClasses });
        },
        options: result.classes.map((c) => ({ value: c, label: c })),
      });
    }

    return configs;
  }, [
    result,
    groupBy,
    target,
    selectedGroupIds,
    selectedClasses,
    replaceListUrl,
  ]);

  // Surface a group/class restriction that the standard dropdowns above cannot
  // show. Those dropdowns only render when the preview exposes more than one
  // option (`result.groups`/`result.classes` are derived from the day's rows).
  // A bookmarked filter can point at a group/class with no rows for the selected
  // date + source: the reconciliation effect deliberately keeps that selection
  // (dropping it would broaden a confidential single-group list to the whole
  // cohort), but with zero or one visible options the control disappears,
  // leaving an unexplained empty preview and no way to inspect or clear the
  // restriction. Render a removable chip for exactly those cases so the still-
  // active filter stays visible and clearable (#1565 review pass 2).
  const hiddenActiveFilters: ActiveFilter[] = useMemo(() => {
    const chips: ActiveFilter[] = [];
    const groupDropdownShown = !!result && result.groups.length > 1;
    const classDropdownShown = !!result && result.classes.length > 1;
    if (selectedGroupIds !== null && !groupDropdownShown) {
      chips.push({
        id: "group-restriction",
        label: "Gruppenfilter aktiv",
        onRemove: () => {
          setSelectedGroupIds(null);
          replaceListUrl({ selectedGroupIds: null });
        },
      });
    }
    if (selectedClasses !== null && !classDropdownShown) {
      chips.push({
        id: "class-restriction",
        label: "Klassenfilter aktiv",
        onRemove: () => {
          setSelectedClasses(null);
          replaceListUrl({ selectedClasses: null });
        },
      });
    }
    return chips;
  }, [result, selectedGroupIds, selectedClasses, replaceListUrl]);

  // Group the preview rows by their backend-assigned section heading. Rows
  // arrive pre-sorted by group, so Map insertion order keeps sections intact.
  const sections = useMemo(() => {
    const rows = result?.rows ?? [];
    const map = new Map<string, SlotListRow[]>();
    for (const row of rows) {
      const key = row.group_title ?? "";
      const bucket = map.get(key) ?? [];
      bucket.push(row);
      map.set(key, bucket);
    }
    return [...map.entries()].map(([title, sectionRows]) => ({
      title,
      rows: sectionRows,
    }));
  }, [result]);

  const columns = useMemo<DataTableColumn<SlotListRow>[]>(() => {
    const cols: DataTableColumn<SlotListRow>[] = [
      {
        key: "name",
        header: "Name",
        render: (r) => <span className="font-medium">{r.name}</span>,
        sortValue: (r) => r.name,
      },
      {
        key: "school_class",
        header: "Klasse",
        render: (r) => r.school_class,
        sortValue: (r) => r.school_class,
      },
      {
        key: "group_name",
        header: "Gruppe",
        render: (r) => r.group_name || "–",
        sortValue: (r) => r.group_name,
      },
    ];
    if (isPickupBased) {
      cols.push({
        key: "pickup_time",
        header: "Abholung",
        render: (r) => r.pickup_time ?? "–",
        sortValue: (r) => r.pickup_time ?? "",
      });
    } else {
      cols.push({
        key: "slot",
        header: "Angebot",
        render: (r) => r.slot,
        sortValue: (r) => r.slot,
      });
    }
    cols.push({
      key: "status",
      header: "Status",
      render: (r) => <StatusBadge row={r} source={source} />,
      sortValue: (r) => r.status_label,
    });
    return cols;
  }, [isPickupBased, source]);

  if (authStatus === "loading" || settingsSchemaLoading) {
    return <Loading fullPage={false} />;
  }

  if (timetableDisabled) {
    return (
      <PlanningDisabledState
        pageTitle="Tageslisten"
        heading="Tageslisten sind deaktiviert"
        description="Tageslisten gehören zum Betreuungsplan, der für diese Schule ausgeschaltet ist. Er kann in den Einstellungen unter „Betrieb“ wieder aktiviert werden."
        testId="slot-lists-disabled-state"
      />
    );
  }

  const selectedOption = SELECTION_OPTIONS.find(
    (option) =>
      option.target === target &&
      (option.target === "pickup_cohort"
        ? option.pickupCohort === pickupCohort
        : (option.listKind ?? "") === listKind),
  );
  const selectedListLabel =
    result?.list_label ??
    (isPickupBased ? pickupOptionByCohort.get(pickupCohort)?.label : null) ??
    selectedOption?.label ??
    "";
  return (
    <main className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 lg:px-8">
      {/* Reached from Datenverwaltung → Exporte (no sidebar entry of its own). */}
      <BackButton referrer="/database/exports" />
      {/* Selection: source + date + data mode */}
      <section
        aria-label="Listenauswahl"
        className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5"
      >
        <p className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Quelle
        </p>
        <div className="grid auto-rows-fr grid-cols-2 gap-2.5 lg:grid-cols-3">
          {SELECTION_OPTIONS.map((option) => {
            const Icon = option.icon;
            const selected =
              option.target === target &&
              (option.target === "pickup_cohort"
                ? option.pickupCohort === pickupCohort
                : (option.listKind ?? "") === listKind);
            const availability = option.pickupCohort
              ? pickupOptionByCohort.get(option.pickupCohort)
              : null;
            const kindAvailability = option.listKind
              ? listOptions?.list_kinds.find(
                  (item) => item.kind === option.listKind,
                )
              : null;
            const label =
              availability?.label ?? kindAvailability?.label ?? option.label;
            const hasAvailability = Boolean(availability ?? kindAvailability);
            const slotCount = listOptions?.slots.length ?? 0;
            const available = option.listKind
              ? (kindAvailability?.available ?? true)
              : option.target === "slots"
                ? slotCount > 0
                : (availability?.available ?? true);
            // A Ganztag pickup cohort on a past date always fails at the backend
            // (ErrPickupCohortPastDate); disable the choice rather than expose an
            // action that only errors. Slot lists remain available for any date.
            const disabledForPastDate =
              option.target === "pickup_cohort" && isPastDate;
            const detail = disabledForPastDate
              ? "Nur für heute und künftige Tage"
              : option.listKind
                ? kindAvailability
                  ? kindAvailability.slot_count > 0
                    ? `${kindAvailability.slot_count} Termin${kindAvailability.slot_count === 1 ? "" : "e"} zugeordnet`
                    : "Keine Termine zugeordnet"
                  : isOptionsLoading
                    ? "Prüfe Datum…"
                    : option.description
                : option.target === "slots"
                  ? isOptionsLoading
                    ? "Prüfe Datum…"
                    : slotCount > 0
                      ? cancelledSlotCount > 0
                        ? `${selectableSlotIds.length} aktiv, ${cancelledSlotCount} abgesagt`
                        : `${slotCount} Angebot${slotCount === 1 ? "" : "e"} geplant`
                      : "Keine Angebote geplant"
                  : availability
                    ? availability.row_count > 0
                      ? `${availability.row_count} Kinder mit passender Abholzeit`
                      : "Keine Kinder mit passender Abholzeit"
                    : isOptionsLoading
                      ? "Prüfe Datum…"
                      : option.description;
            return (
              <button
                key={option.id}
                type="button"
                onClick={() => pickSelection(option)}
                disabled={disabledForPastDate}
                aria-pressed={selected}
                title={
                  disabledForPastDate
                    ? "Ganztag-Listen sind nur für heute und künftige Tage verfügbar"
                    : undefined
                }
                className={`flex h-full items-center gap-3 rounded-xl border p-3 text-left transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 ${
                  option.id === "slots" ? "col-span-2 lg:col-span-3" : ""
                } ${
                  selected
                    ? "border-[#83CD2D]/50 bg-[#83CD2D]/10"
                    : "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50"
                }`}
              >
                <span
                  className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-gray-100 ${
                    selected
                      ? "bg-[#83CD2D]/20 text-[#3F6F12]"
                      : available || !hasAvailability
                        ? "bg-white text-gray-500"
                        : "bg-white text-gray-400"
                  }`}
                >
                  <Icon className="h-5 w-5" aria-hidden />
                </span>
                <span className="min-w-0">
                  <span className="block truncate text-sm font-semibold text-gray-950">
                    {label}
                  </span>
                  <span className="block truncate text-xs text-gray-500">
                    {detail}
                  </span>
                </span>
              </button>
            );
          })}
        </div>

        {isManualSlotSelection ? (
          <div className="mt-3 border-t border-gray-100 pt-3">
            <div className="mb-2 grid min-h-7 grid-cols-[auto_1fr_auto] items-center gap-2">
              <span className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Enthaltene Angebote
              </span>
              <span className="min-w-0 truncate text-xs text-gray-500">
                {slotSummary}
              </span>
              <button
                type="button"
                onClick={() => {
                  setSelectedSlotIds(null);
                  replaceListUrl({ selectedSlotIds: null });
                }}
                tabIndex={selectedSlotIds === null ? -1 : 0}
                aria-hidden={selectedSlotIds === null}
                className={`rounded-md px-2 py-1 text-xs font-medium text-[#315C9B] transition-colors hover:bg-[#5080D8]/8 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                  selectedSlotIds === null
                    ? "pointer-events-none opacity-0"
                    : "opacity-100"
                }`}
              >
                Alle auswählen
              </button>
            </div>
            {slotOptions.length > 0 ? (
              <div className="grid max-h-[11.5rem] grid-cols-3 gap-2 overflow-y-auto pr-1">
                {slotOptions.map((slot) => {
                  const cancelled = slot.status === CANCELLED_SLOT_STATUS;
                  const selected =
                    !cancelled && selectedSlotIdSet.has(slot.instance_id);
                  return (
                    <label
                      key={slot.instance_id}
                      title={`${slot.time_range} ${slot.title}`}
                      className={`flex h-14 min-w-0 items-center gap-3 rounded-xl border px-3 py-2.5 text-sm font-medium transition-colors focus-within:border-gray-400 ${
                        cancelled
                          ? "cursor-not-allowed border-gray-100 bg-gray-50 text-gray-400"
                          : selected
                            ? "cursor-pointer border-gray-300 bg-gray-50 text-gray-700"
                            : "cursor-pointer border-gray-100 bg-white text-gray-700 hover:border-gray-200 hover:bg-gray-50"
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={selected}
                        disabled={cancelled}
                        onChange={() => toggleSlot(slot.instance_id)}
                        className="sr-only"
                      />
                      <span
                        className={`flex h-5 w-5 shrink-0 items-center justify-center rounded-md border transition-colors ${
                          selected
                            ? "border-gray-900 bg-gray-900"
                            : "border-gray-300 bg-white"
                        }`}
                        aria-hidden="true"
                      >
                        <Check
                          className={`h-3.5 w-3.5 text-white transition-opacity ${
                            selected ? "opacity-100" : "opacity-0"
                          }`}
                        />
                      </span>
                      <span className="min-w-0 flex-1 leading-snug">
                        <span className="block truncate">{slot.title}</span>
                        <span className="block truncate text-xs font-normal text-gray-500 tabular-nums">
                          {slot.time_range}
                          {cancelled ? " · Abgesagt" : ""}
                        </span>
                      </span>
                    </label>
                  );
                })}
              </div>
            ) : (
              <p className="text-sm text-gray-500">
                Für dieses Datum sind keine Angebote geplant.
              </p>
            )}
            {cancelledSlotCount > 0 ? (
              <p className="mt-2 text-xs leading-5 text-gray-500">
                Abgesagte Angebote werden angezeigt, aber nicht in Tageslisten
                aufgenommen.
              </p>
            ) : null}
          </div>
        ) : null}

        {/* Compact controls row — no dead space, provenance pinned right */}
        <div className="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-4 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-6 sm:gap-y-3">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-gray-700">Datum</span>
            <DatePicker
              value={parseISODate(dateISO)}
              onChange={pickDate}
              placeholder="Datum"
              hideClearButton
              minDate={pickerMinDate}
              maxDate={pickerMaxDate}
            />
          </div>
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium text-gray-700">
              Datenbasis
            </span>
            <Tabs
              value={source}
              onValueChange={(v) => {
                const nextSource = v as SlotListSource;
                setSource(nextSource);
                setSelectedGroupIds(null);
                setSelectedClasses(null);
                // Abgleich rejects future dates (ErrReconciliationFutureDate).
                // The picker's maxDate only blocks the next pick, not an
                // already-selected future day, so clamp to the school's Berlin
                // today during this same transition; otherwise the next preview
                // is backend-rejected.
                const clampFutureDate =
                  nextSource === "reconciliation" && dateISO > berlinTodayISO();
                const nextDate = clampFutureDate ? berlinTodayISO() : dateISO;
                if (clampFutureDate) {
                  setDateISO(nextDate);
                  setSelectedSlotIds(null);
                }
                replaceListUrl({
                  source: nextSource,
                  ...(clampFutureDate
                    ? { dateISO: nextDate, selectedSlotIds: null }
                    : {}),
                  selectedGroupIds: null,
                  selectedClasses: null,
                });
              }}
            >
              <TabsList variant="default">
                {SOURCES.map((s) => (
                  <TabsTrigger key={s} value={s}>
                    {SLOT_LIST_SOURCE_LABELS[s]}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </div>
          {result ? (
            <p className="text-xs leading-5 text-gray-500 sm:ml-auto sm:max-w-[18rem] sm:text-right">
              Datenherkunft: {result.provenance}
            </p>
          ) : null}
        </div>
      </section>

      {error ? (
        <div className="mt-4 rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/10 p-3 text-sm text-[#FF3130]">
          {error}
        </div>
      ) : null}

      {/* Preview + export */}
      <section
        aria-label="Vorschau und Export"
        className="moto-content-surface mt-4 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-5"
      >
        <div className="space-y-3 border-b border-gray-100 pb-4">
          <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
            <div className="min-w-0 space-y-3">
              {/* Standard filters that narrow the list (offering / group / class) */}
              {filterConfigs.length > 0 ? (
                <DesktopFilters filters={filterConfigs} />
              ) : null}
              {hiddenActiveFilters.length > 0 ? (
                <ActiveFilterChips filters={hiddenActiveFilters} />
              ) : null}
              {result && !isLoading ? (
                <CounterChips counters={result.counters} source={source} />
              ) : null}
            </div>
            <div className="flex flex-wrap gap-2 lg:justify-end">
              <Button
                type="button"
                variant="primary"
                size="sm"
                isLoading={isExporting}
                loadingText="Erstelle PDF…"
                disabled={isExporting || isLoading || !result}
                onClick={() => void handleExport("pdf", "print")}
                className="gap-2"
              >
                <Printer className="h-4 w-4" aria-hidden />
                Drucken
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isExporting || isLoading || !result}
                onClick={() => void handleExport("pdf", "download")}
                className="gap-2"
              >
                <Download className="h-4 w-4" aria-hidden />
                PDF herunterladen
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={isExporting || isLoading || !result}
                onClick={() => void handleExport("xlsx", "download")}
                className="gap-2"
              >
                <FileSpreadsheet className="h-4 w-4" aria-hidden />
                XLSX
              </Button>
            </div>
          </div>
        </div>

        <div className="mt-4">
          {groupBy && !isLoading && (result?.rows.length ?? 0) > 0 ? (
            <div className="space-y-5">
              {sections.map((section) => (
                <div key={section.title}>
                  <h3 className="mb-2 text-sm font-semibold text-gray-700">
                    {section.title}{" "}
                    <span className="font-normal text-gray-400">
                      ({section.rows.length})
                    </span>
                  </h3>
                  <DataTable
                    columns={columns}
                    rows={section.rows}
                    getRowKey={(r) =>
                      `${r.instance_id ?? ""}-${r.slot}-${r.student_id}`
                    }
                  />
                </div>
              ))}
            </div>
          ) : (
            <DataTable
              columns={columns}
              rows={result?.rows ?? []}
              getRowKey={(r) =>
                `${r.instance_id ?? ""}-${r.slot}-${r.student_id}`
              }
              isLoading={isLoading}
              pageSize={PREVIEW_PAGE_SIZE}
              emptyState={
                <div className="py-8 text-center text-gray-500">
                  {isPickupBased
                    ? "Keine Kinder mit passender Abholzeit an diesem Tag."
                    : listKind
                      ? `Keine Kinder für „${selectedListLabel}“ an diesem Tag.`
                      : `Keine Kinder für „${selectedListLabel}“ an diesem Tag. Wähle oben ein oder mehrere Angebote.`}
                </div>
              }
            />
          )}
        </div>
      </section>
    </main>
  );
}
