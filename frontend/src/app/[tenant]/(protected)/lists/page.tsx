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
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { DesktopFilters } from "~/components/ui/page-header/DesktopFilters";
import { ActiveFilterChips } from "~/components/ui/page-header/ActiveFilterChips";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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
  SlotListExportError,
  SlotListExportSupersededError,
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
  // The Ganztag pickup boundaries are tenant-configurable (config.setting_values
  // slot_list_*_day_cutoff), so these label/description strings are only neutral
  // placeholders shown before the date's /options response arrives. Once it does,
  // the render sources the concrete boundary from pickup_cohorts[].label — never
  // these defaults — so a school on non-default cutoffs never sees a wrong time
  // (#1565 review pass 1).
  {
    id: "short_day",
    target: "pickup_cohort",
    pickupCohort: "short_day",
    label: "Ganztag (frühe Abholung)",
    description: "Frühere Abholzeit",
    icon: Clock3,
  },
  {
    id: "long_day",
    target: "pickup_cohort",
    pickupCohort: "long_day",
    label: "Ganztag (späte Abholung)",
    description: "Spätere Abholzeit",
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
  if (row.planned) return LOCATION_COLORS.DANGER;
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
              color: LOCATION_COLORS.DANGER,
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

// nextSelection collapses a multi-select to null ("all", no restriction) ONLY
// when the chosen set genuinely covers every current option and carries no
// hidden extra; otherwise it keeps the subset.
//
// A bookmarked `gruppen`/`klassen` filter can carry a value that is absent from
// today's options — a group/class that simply has no rows for the selected date
// + source (result.groups/result.classes are derived from the preview rows, not
// the school's full set). Such a HIDDEN value still rides the dropdown's current
// selection, and it MUST be preserved: dropping it and then collapsing an emptied
// set to null would silently lift a confidential restriction and broaden the
// named-student list to the whole cohort. Concretely, with visible group A plus
// hidden group B selected, deselecting A leaves `values === [B]`; filtering B out
// as "unknown" would empty the set and return null (no restriction), disclosing
// every group. So keep unknown selected values verbatim — they clear only on an
// explicit filter reset — and only report "no restriction" when every current
// option is chosen AND nothing hidden remains (#1565 review pass 1). Keying on
// membership rather than a bare `values.length === total` count also stops a
// partial selection that happens to sit alongside a hidden value from being
// mis-read as "complete".
function nextSelection<T>(values: T[], options: T[]): T[] | null {
  const optionSet = new Set(options);
  const deduped = [...new Set(values)];
  const known = deduped.filter((value) => optionSet.has(value));
  const unknown = deduped.filter((value) => !optionSet.has(value));
  // Every current option chosen and no hidden selection left → truly all groups.
  if (known.length === optionSet.size && unknown.length === 0) return null;
  const chosen = [...known, ...unknown];
  if (chosen.length === 0) return null; // nothing selected at all → no restriction
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

// Unordered membership equality — the backend treats instance_ids as a set, so a
// mere reordering of /options slots is not a meaningful change. Used to decide
// whether a since-loaded options refresh actually altered the selectable set.
function sameStringSet(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const set = new Set(a);
  return b.every((value) => set.has(value));
}

// A stable content signature of an options payload, order-independent. A
// background revalidation only swaps in fresh options (and thus re-fires the
// preview) when this signature changes, so tabbing back to an unchanged page
// costs one silent fetch and no redundant preview/BuildList churn (#1565 review
// pass 2 — stale one-time /options snapshot).
function optionsSignature(options: SlotListOptionsResult): string {
  // Encode each slot as a JSON tuple, not a `:`-joined string: a slot's
  // title/time_range are free text (a time_range like "14:30 - 15:15" already
  // contains the delimiter), so plain concatenation could let two genuinely
  // different payloads collapse to the same signature and miss a rename/
  // reschedule. title and time_range MUST be part of the signature — when a
  // slot is renamed or moved in another tab its ID and status are unchanged,
  // and without these fields the fresh payload is discarded on focus while a
  // later export silently renders the updated label (#1565 review pass 3).
  // list_kind and room_name matter for the same reason (#1565 review pass 4):
  // a list_kind reassignment moves a slot in/out of a classified list and a
  // room change reshapes a room-grouped export, both without touching the
  // active instance-ID set — omit them and the drift slips past the guard.
  const slots = options.slots
    .map((slot) =>
      JSON.stringify([
        slot.instance_id,
        slot.status,
        slot.title,
        slot.time_range,
        slot.list_kind ?? "",
        slot.room_name ?? "",
      ]),
    )
    .sort((a, b) => a.localeCompare(b))
    .join(",");
  const cohorts = options.pickup_cohorts
    .map(
      (cohort) =>
        `${cohort.cohort}:${cohort.available ? 1 : 0}:${cohort.row_count}`,
    )
    .sort((a, b) => a.localeCompare(b))
    .join(",");
  const kinds = options.list_kinds
    .map(
      (kind) =>
        `${kind.kind}:${kind.available ? 1 : 0}:${kind.slot_count}:${kind.row_count}`,
    )
    .sort((a, b) => a.localeCompare(b))
    .join(",");
  return `${slots}|${cohorts}|${kinds}`;
}

// The active (non-cancelled) instance IDs of an options payload — the same
// derivation activeInstanceIds performs on listOptions, extracted so the
// pre-export guard can recompute it against a freshly fetched payload.
function activeInstanceIdsOf(options: SlotListOptionsResult): string[] {
  return options.slots
    .filter((slot) => slot.status !== CANCELLED_SLOT_STATUS)
    .map((slot) => slot.instance_id);
}

// A stable, order-independent signature of the pickup-cohort metadata — cohort
// key, label (which embeds the tenant's cutoff, e.g. "Ganztag bis 14:30"),
// availability and row count. A pickup list carries no instance_ids or slot
// metadata, so the slot-oriented guard skips it; this lets the pre-export guard
// still catch a cutoff change made after the preview rendered. Moving a cutoff
// re-buckets which children fall into short/long day AND rewrites the cohort's
// header, so exporting against the stale snapshot would hand out a different
// cohort and label than the reviewed preview (#1565 review pass 4). JSON tuples
// keep the free-text label delimiter-safe, mirroring the slot signature.
function pickupCohortsSignature(options: SlotListOptionsResult): string {
  return options.pickup_cohorts
    .map((cohort) =>
      JSON.stringify([
        cohort.cohort,
        cohort.label,
        cohort.available ? 1 : 0,
        cohort.row_count,
      ]),
    )
    .sort((a, b) => a.localeCompare(b))
    .join(",");
}

// Metadata signature of ONLY the slots that appear in the requested slot-list
// document — never the whole day. optionsSignature covers every slot, pickup
// cohort and list-kind aggregate for the date, so as a pre-export drift guard it
// trips on changes to unrelated slots/cohorts/kinds and refuses an otherwise
// identical export (#1565 review pass 5). Scope the comparison to the effective
// set:
//   - Classified list (listKind set): the active slots whose list_kind is the
//     requested kind. A reassignment that moves a slot into or out of that kind
//     changes set membership here — caught — even though the active instance-ID
//     set the backend intersects against is unchanged, so list_kind need not be
//     in the tuple.
//   - Manual selection (listKind ""): the active slots the user selected, or
//     every active slot when the selection is "all active" (selectedIds null).
// A rename, reschedule or room change of an in-scope slot changes its tuple and
// registers as drift; the same change on an out-of-scope slot does not. JSON
// tuples keep the free-text title/time_range/room_name delimiter-safe, mirroring
// optionsSignature.
function slotDocumentSignature(
  options: SlotListOptionsResult,
  listKind: SlotListKind | "",
  selectedIds: Set<string> | null,
): string {
  return options.slots
    .filter((slot) => {
      if (slot.status === CANCELLED_SLOT_STATUS) return false;
      if (listKind) return (slot.list_kind ?? "") === listKind;
      return selectedIds === null || selectedIds.has(slot.instance_id);
    })
    .map((slot) =>
      JSON.stringify([
        slot.instance_id,
        slot.status,
        slot.title,
        slot.time_range,
        slot.room_name ?? "",
      ]),
    )
    .sort((a, b) => a.localeCompare(b))
    .join(",");
}

// A stable signature of a SINGLE pickup cohort — the one being exported. The
// export re-buckets and re-labels only the requested cohort, so comparing the
// whole pickup_cohorts array (pickupCohortsSignature) as a pre-export drift
// guard would refuse the export when an unrelated cohort's cutoff or roster
// changed (#1565 review pass 5). An absent cohort collapses to "" — a real
// signature is a JSON array string, so this sentinel never collides with one.
function pickupCohortSignature(
  options: SlotListOptionsResult,
  cohort: SlotListPickupCohort,
): string {
  const match = options.pickup_cohorts.find((c) => c.cohort === cohort);
  if (!match) return "";
  return JSON.stringify([
    match.cohort,
    match.label,
    match.available ? 1 : 0,
    match.row_count,
  ]);
}

// A full content signature of a rendered preview — list label, counters and
// every row's identity, placement and status. The /options metadata guards
// (slotDocumentSignature / pickupCohortSignature) only see slot and cohort
// structure; roster membership and attendance can change without touching them
// (a child swaps groups or cohorts, checks in/out, is marked sick), yet the
// export endpoint re-derives from current data. Comparing this signature of a
// freshly refetched preview against the one the user reviewed catches exactly
// that drift, so an export never hands out names or statuses the reviewed list
// never showed (#1565 review pass 2). Rows are compared positionally: the
// preview and the export both sort identically (BuildList sortRows), so element
// order is stable and a positional tuple is enough.
function resultSignature(result: SlotListResult): string {
  return JSON.stringify({
    label: result.list_label,
    counters: result.counters,
    rows: result.rows.map((row) => [
      row.student_id,
      row.name,
      row.school_class,
      row.group_name,
      row.group_id ?? "",
      row.instance_id ?? "",
      row.slot,
      row.room_name ?? "",
      row.pickup_time ?? "",
      row.status_label,
      row.planned,
      row.present,
      row.unplanned,
      row.excused,
      row.group_title ?? "",
    ]),
  });
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
  const { data: settingsSchema } = useSettingsSchema(
    authStatus === "authenticated",
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
    },
  );
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
  // Bumped on focus/visibility to force the preview effect to refetch. The
  // /options revalidation alone cannot catch attendance drift: its payload holds
  // only slot, pickup-cohort and planned-roster metadata, so a check-in/out or a
  // per-slot status change leaves listOptions' signature unchanged and the
  // preview — keyed on request/listOptions — would otherwise keep rendering stale
  // present/missing rows and counters until an unrelated filter changes (#1565
  // review pass 2).
  const [previewNonce, setPreviewNonce] = useState(0);
  // A warning that must survive the very next preview refetch. The preview
  // effect clears any error at the start of its run, so an export-time drift
  // message set alongside a previewNonce bump would only flash and vanish before
  // the user could read it. Stash it here instead; the effect consumes it once
  // the fresh preview has landed, so the user sees the refreshed data AND the
  // "please review and retry" notice together (#1565 review pass 8).
  const pendingPreviewWarning = useRef<string | null>(null);

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
    () => (listOptions ? activeInstanceIdsOf(listOptions) : null),
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

  // A live snapshot of the request identity, refreshed every render. handleExport
  // closes over the request value at click time and then awaits two network
  // round-trips before it hands out the file; during that window the date,
  // source, list type, slot selection, or filters can still change. This ref
  // exposes the CURRENT request so the export can detect that mid-flight change
  // and refuse rather than download/print the list the user has already navigated
  // away from — the revalidation fetches below all use the captured (now stale)
  // request/dateISO, so they validate the old document against itself and would
  // otherwise pass. The request useMemo returns a stable reference while its
  // document-affecting inputs are unchanged and a fresh one on any change, so an
  // identity comparison against the captured value is exact (#1565 review pass 7).
  const requestRef = useRef(request);
  // Sync the ref in the COMMIT phase — not during render, not in a passive effect.
  // handleExport re-reads requestRef.current at its identity gates after awaiting
  // two network round-trips, so the ref must reflect the request the user has
  // actually committed to:
  //   - A render-phase write (`requestRef.current = request`) also publishes
  //     SPECULATIVE renders: React's concurrent renderer can begin rendering a
  //     filter/date change and then abandon or supersede it, but a ref mutation is
  //     never rolled back — so an export identity gate could observe a request the
  //     user never navigated to and falsely discard a valid file.
  //   - A passive effect (useEffect) runs only after paint, leaving a gap between
  //     commit and the effect in which a resuming fetch continuation would still
  //     read the pre-change request and export the list the UI has already left.
  // useLayoutEffect runs synchronously inside commit — after a render is actually
  // committed (never for a discarded speculative render) and before the browser
  // yields to a microtask / fetch continuation — so it closes both windows.
  // `request` is a stable useMemo, so this only stores a new reference when a
  // document-affecting input actually changes and the identity comparison against
  // the captured value stays exact (#1565 review pass 3 follow-up).
  useLayoutEffect(() => {
    requestRef.current = request;
  }, [request]);

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
    const generation = ++optionsRequestGenerationRef.current;
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
        if (cancelled) return;
        // A background revalidation may have started after this initial load and
        // already installed a newer schedule snapshot while listOptions was
        // still null; drop this now-stale result rather than clobbering it
        // (#1565 review pass 2).
        if (generation <= optionsCommittedGenerationRef.current) return;
        optionsCommittedGenerationRef.current = generation;
        setListOptions(options);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("slot_list_options_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        // A newer request already succeeded — do not overwrite good options with
        // this superseded load's error. (A failure never advances the marker, so
        // it cannot block a later revalidation from recovering the page.)
        if (generation <= optionsCommittedGenerationRef.current) return;
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

  // The current date, mirrored into a ref so a background /options revalidation
  // that resolves after the user has moved to another day can detect the switch
  // and drop its now-irrelevant result instead of overwriting the new date's
  // options.
  const dateISORef = useRef(dateISO);
  useEffect(() => {
    dateISORef.current = dateISO;
  }, [dateISO]);

  // A monotonic id stamped on EVERY /options request — the initial date-load
  // fetch AND each background revalidation increment the SAME counter. Focus and
  // visibilitychange can both fire on one return-to-tab, and a revalidation can
  // start while the initial load is still in flight; a date-only guard would then
  // let whichever RESOLVES last win, so an earlier request that read the pre-edit
  // schedule could clobber a newer one and roll the preview back to stale
  // slots/cutoffs — or the initial load could overwrite a newer revalidation that
  // resolved first while listOptions was still null (#1565 review pass 2). Each
  // request captures its id at START; on resolve it commits only when its id is
  // newer than optionsCommittedGenerationRef, then advances that marker, so a
  // later-resolving-but-older request is dropped regardless of resolution order.
  // A FAILED request commits nothing and does NOT advance the marker, so it never
  // permanently blocks a still-valid sibling (a retry / a focus revalidation) from
  // installing — which is what recovers a page whose initial load failed.
  const optionsRequestGenerationRef = useRef(0);
  const optionsCommittedGenerationRef = useRef(0);

  // The /options snapshot is otherwise fetched only when the date changes, so a
  // page left open across the day would keep sending a stale active-slot set to
  // every preview and export — omitting a slot added since load and retaining a
  // slot cancelled since load (#1565 review pass 2). Revalidate quietly whenever
  // the tab/window regains focus (the realistic "edited the plan elsewhere, came
  // back here" path). This is a BACKGROUND refresh: it never nulls listOptions
  // (which would flash the spinner and disable the slot controls) and only swaps
  // in the fresh payload when its content signature actually changed — feeding
  // activeInstanceIds, which re-fires the preview and prunes now-cancelled manual
  // selections. A failed refresh is non-fatal: the last-known options stay, and
  // the pre-export guard still revalidates before any list leaves the building.
  const revalidateOptions = useCallback(() => {
    if (authStatus !== "authenticated" || timetableDisabled) return;
    const requestedDate = dateISO;
    const generation = ++optionsRequestGenerationRef.current;
    fetchSlotListOptions(requestedDate)
      .then((fresh) => {
        if (requestedDate !== dateISORef.current) return; // date moved on
        // Superseded by a newer /options request — the initial load, a later
        // revalidation, or a date change — that already committed its result.
        // Drop this one no matter which network round-trip finished first.
        if (generation <= optionsCommittedGenerationRef.current) return;
        optionsCommittedGenerationRef.current = generation;
        setOptionsError(null);
        setListOptions((prev) => {
          // Newest request and nothing installed yet: the initial load is still
          // in flight or FAILED. Install the fresh payload outright — the shared
          // generation marker guarantees the pending initial load, if it resolves
          // later, will not overwrite this newer snapshot, and recovers a page
          // whose initial load failed (#1565 review pass 2 / pass 4).
          if (!prev) return fresh;
          // Reinstall on ANY change a downstream reader renders, INCLUDING a
          // pickup cutoff moved without re-bucketing any child. optionsSignature
          // keys cohorts on availability + row_count only, so a label-only cutoff
          // change ("Ganztag bis 14:30" -> "bis 14:00", same row_count) slips past
          // it — listOptions is not reinstalled, the pickup preview never re-fires,
          // and its cohort header stays on the old cutoff until an export attempt
          // trips the export drift guard. Fold the cohort-label signature in here
          // so focus revalidation reinstalls on a label-only change too, re-firing
          // the preview so the visible cohort matches what an export would produce
          // (#1565 review pass 1). This whole-payload comparison is intentional
          // here — it only decides whether to re-render the preview, never blocks
          // anything — whereas the export drift guard scopes its check to the
          // requested list (slotDocumentSignature / pickupCohortSignature) so an
          // unrelated change can never spuriously refuse an export.
          const unchanged =
            optionsSignature(prev) === optionsSignature(fresh) &&
            pickupCohortsSignature(prev) === pickupCohortsSignature(fresh);
          return unchanged ? prev : fresh;
        });
      })
      .catch((err: unknown) => {
        logger.warn("slot_list_options_revalidate_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [authStatus, dateISO, timetableDisabled]);

  useEffect(() => {
    if (authStatus !== "authenticated" || timetableDisabled) return;
    // Refresh BOTH the options snapshot (slot/cohort structure) and the preview
    // (live attendance / roster). revalidateOptions only re-fires the preview
    // when the options signature changes; a pure attendance change does not touch
    // it, so bump previewNonce to force a preview refetch on every focus/return
    // (#1565 review pass 2).
    //
    // Restoring a background tab commonly fires BOTH visibilitychange→visible AND
    // focus, so without coalescing each handler would run refresh() and issue a
    // duplicate /options + /preview pair — redundant slot/roster scans that also
    // contend on the cutoff lock. The generation counter downstream only discards
    // the stale client commit; it does not cancel the duplicated backend work.
    // Collapse any burst of events into a single trailing refresh so the pair
    // becomes one request set (#1565 review).
    let coalesce: ReturnType<typeof setTimeout> | null = null;
    const refresh = () => {
      if (coalesce !== null) return;
      coalesce = setTimeout(() => {
        coalesce = null;
        revalidateOptions();
        setPreviewNonce((n) => n + 1);
      }, 100);
    };
    const onFocus = () => refresh();
    const onVisibility = () => {
      if (document.visibilityState === "visible") refresh();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      if (coalesce !== null) clearTimeout(coalesce);
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [authStatus, timetableDisabled, revalidateOptions]);

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
        if (cancelled) return;
        setResult(preview);
        // Re-apply a warning stashed by the export drift path (409 / header-only
        // drift) after the fresh preview lands, so it survives the setError(null)
        // above instead of flashing (#1565 review pass 8). Deliberately re-read
        // the LIVE ref (not a value captured at effect start) and do NOT clear it
        // here: a single 409 fires BOTH an options revalidation and a nonce bump,
        // so the preview effect can run more than once before the refresh settles.
        // Consuming the warning in the first run to resolve let the later
        // options-triggered run — which supersedes it — land with an empty ref and
        // wipe the message. Keeping the ref set lets whichever run settles last
        // re-apply it; it is cleared only when the user starts a new export
        // (handleExport) or switches context (the effect below) (#1565 review pass
        // 9 / pass 10).
        if (pendingPreviewWarning.current) {
          setError(pendingPreviewWarning.current);
        }
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
    // A focus/visibility refresh bumps this to force a preview refetch even when
    // request and listOptions are unchanged but attendance may have drifted.
    previewNonce,
  ]);

  // A stashed export-drift warning stays applicable only as long as the user is
  // still looking at the SAME document that produced the drift. The dependency
  // list is every user-controlled input that changes which document the export
  // covers: not just date/target/source/pickup cohort/list kind, but also the
  // grouping, the manual slot selection, and the group/class filters — each of
  // those produces a different document and a fresh preview, after which the old
  // "please re-export" notice is stale and must clear (#1565 review pass 12).
  //
  // Excluded on purpose is the background options revalidation the 409 path fires:
  // it touches listOptions/activeInstanceIds (NOT these user inputs), so the
  // warning still survives that refresh — the whole point of stashing it. Slot
  // pruning can rewrite selectedSlotIds during that refresh, but only when a
  // selected slot genuinely vanished, which is itself a real document change.
  useEffect(() => {
    pendingPreviewWarning.current = null;
  }, [
    dateISO,
    target,
    source,
    pickupCohort,
    listKind,
    groupBy,
    selectedSlotIds,
    selectedGroupIds,
    selectedClasses,
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

      // Open the print placeholder NOW, synchronously, while the click's
      // transient user activation is still valid. The slot revalidation below
      // awaits a network round-trip; after it Safari and strict popup blockers
      // return null from window.open, so a tab opened inside exportSlotList
      // would fail and Drucken would never start (#1565 review pass 3). Hand the
      // retained tab to exportSlotList; close it on every bail-out path so a
      // refused export leaves no orphaned blank tab.
      let printTarget: Window | null = null;
      if (mode === "print") {
        printTarget = globalThis.open("", "_blank");
        if (!printTarget) {
          setError("Der Druckdialog konnte nicht geöffnet werden.");
          return;
        }
      }

      // Lock the export action NOW, before the two async revalidation round-trips
      // below, so a double-click cannot launch duplicate downloads/print tabs.
      // isExporting only disables the export buttons, though — the date, source,
      // list-type, slot and filter controls stay interactive during preflight
      // (disabling them would mean threading a disabled prop through the shared
      // DatePicker/DesktopFilters kit). Changing any of them mid-preflight is
      // instead caught by the requestRef identity gates below, which abort before
      // a stale request is handed out (#1565 review pass 7). Every bail-out path
      // below clears the lock again.
      setIsExporting(true);
      setError(null);
      // A fresh export attempt supersedes any lingering "please re-export" drift
      // warning from a previous 409: drop it so a later preview settle cannot
      // re-apply a now-irrelevant message over this attempt's result (#1565
      // review pass 10).
      pendingPreviewWarning.current = null;

      // An export is the one non-idempotent, hand-it-out action here, so before
      // it fires validate against a FRESH /options read — never the possibly-
      // hours-old snapshot the current request/preview was built from (#1565
      // review pass 2 + pass 4). BOTH list types re-resolve their document from
      // current state at export time, so both must re-check here:
      //   - Slot lists: refuse when the effective instance_ids change
      //     (additions/cancellations) OR the slot metadata does — a rename/
      //     reschedule, a room change, or a list_kind reassignment that moves a
      //     slot in/out of a classified list without changing the active ID set
      //     (the backend intersects list_kind with instance_ids, so
      //     request.instance_ids stays equal yet the rendered document differs).
      //   - Pickup cohorts carry no instance_ids/slot metadata, but their cutoffs
      //     are tenant config the export endpoint re-resolves: moving a cutoff
      //     re-buckets the exported children and rewrites the cohort header.
      //     Refuse when the pickup-cohort metadata changed.
      // On any such drift swap in the fresh options (updating the slot/cohort
      // controls and pruning now-cancelled selections) and ask the user to
      // re-check and export again, rather than hand out a stale or
      // differently-labeled list.
      // Stamp this preflight /options read with the SHARED options generation so
      // it participates in the same last-started-wins ordering as the initial
      // load and revalidateOptions. Captured at fetch START (not at commit) so a
      // focus/visibility revalidation already in flight is ordered by when it
      // started, not when it resolves. Without this, the setListOptions(fresh)
      // install in the drift branch below bypasses optionsCommittedGenerationRef
      // entirely: an older revalidation resolving afterwards would still pass its
      // own `generation <= committed` check and overwrite the export-fresh
      // snapshot, rolling active slot IDs or cutoff metadata back and re-firing a
      // stale preview / repeated export refusal (#1565 review pass 1 P2).
      const exportOptionsGeneration = ++optionsRequestGenerationRef.current;
      // The no-drift path below commits this preflight's options generation but,
      // because its drift check is scoped to the list being exported, cannot
      // install `fresh` inline: doing so would churn activeInstanceIds -> request
      // and trip the identity gates further down, refusing an otherwise-valid
      // export for a change OUTSIDE this document. So it STAGES the fresh whole-day
      // snapshot here and commitRefreshedOptions installs it once the attempt has
      // run and can no longer be disrupted by the resulting request change. Without
      // this, advancing the committed marker without storing `fresh` would strand
      // listOptions on the stale snapshot — a Lernzeit slot cancelled while Mensa is
      // exported leaves drift false, yet an older focus refresh is then dropped
      // (generation <= committed), so switching to the affected list before the next
      // refresh re-derives activeInstanceIds from stale data and sends the wrong
      // active IDs (#1565 review pass 2 P2 follow-up).
      let refreshedOptions: SlotListOptionsResult | null = null;
      const commitRefreshedOptions = () => {
        // `fresh` is the whole-day /options snapshot for the captured date; install
        // it as long as the user is still on THAT date (a list-kind / source /
        // filter / slot change keeps it valid — only a date change invalidates it,
        // and that already has its own fresh load). Guarding on the date rather than
        // the full request identity lets a mid-export list switch still receive the
        // fresh active-slot set instead of the stale one.
        //
        // Also require that this preflight's generation is STILL the newest
        // committed one. The no-drift path below advances the shared marker to
        // exportOptionsGeneration when it stages `fresh`; but if a focus/visibility
        // revalidation STARTED after this preflight, it carries a higher generation
        // and can commit a newer /options payload while the preview or export awaits
        // — advancing the marker past exportOptionsGeneration. Installing the older
        // staged snapshot then, purely because the date still matches, would roll
        // that newer payload back (e.g. a cancellation the newer refresh committed
        // reappears) and leave the page on stale active IDs until yet another
        // refresh. When a newer request has won, keep its snapshot and skip the
        // stale install (#1565 review pass 2 P2 follow-up 2).
        if (
          refreshedOptions &&
          dateISORef.current === dateISO &&
          optionsCommittedGenerationRef.current === exportOptionsGeneration
        ) {
          setListOptions(refreshedOptions);
        }
        refreshedOptions = null;
      };
      let fresh: SlotListOptionsResult;
      try {
        fresh = await fetchSlotListOptions(dateISO);
      } catch (err: unknown) {
        printTarget?.close();
        setIsExporting(false);
        logger.error("slot_list_options_revalidate_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(
          "Die Angebote konnten vor dem Export nicht überprüft werden. Bitte versuchen Sie es erneut.",
        );
        return;
      }
      // If the user changed the date, source, list type, slot selection, or a
      // filter while this /options round-trip was in flight, `fresh` describes the
      // OLD document. Everything below — the drift comparison, the
      // setListOptions(fresh) install, the preview refetch and the export —
      // operates on the captured (now stale) request, so installing `fresh` here
      // would overwrite the new date's already-committed options with the old
      // date's snapshot and query the wrong slots. Gate BEFORE any of that state
      // mutation, not just before the export (#1565 review pass 7 / pass 10).
      if (requestRef.current !== request) {
        printTarget?.close();
        setIsExporting(false);
        setError(
          "Die Auswahl hat sich während des Exports geändert. Bitte prüfen und erneut exportieren.",
        );
        return;
      }
      let drift: boolean;
      if (target === "slots") {
        // Scope the metadata drift check to the slots that actually appear in
        // THIS document (the selected slots, or the requested list_kind), so a
        // rename/reschedule/room change on an unrelated slot — or any pickup or
        // other-list-kind change — no longer refuses an unchanged export (#1565
        // review pass 5).
        const selectedIds =
          isManualSlotSelection && selectedSlotIds !== null
            ? new Set(selectedSlotIds)
            : null;
        // The active (non-cancelled) instance IDs THIS document contains, scoped
        // exactly like slotDocumentSignature: a classified list to its own
        // list_kind, a manual selection to the chosen slots. Comparing the whole
        // day's active set (freshActive vs request.instance_ids, which for a
        // classified list is EVERY active slot) let a slot added or cancelled in
        // an unrelated list kind refuse an otherwise-unchanged classified export
        // (#1565 review pass 5 follow-up). For a manual selection this equals the
        // prior `selectedSlotIds ∩ active` set the request already sent.
        const documentActiveIds = (opts: SlotListOptionsResult): string[] =>
          opts.slots
            .filter((slot) => slot.status !== CANCELLED_SLOT_STATUS)
            .filter((slot) =>
              listKind ? (slot.list_kind ?? "") === listKind : true,
            )
            .filter(
              (slot) =>
                selectedIds === null || selectedIds.has(slot.instance_id),
            )
            .map((slot) => slot.instance_id);
        const metadataChanged =
          !listOptions ||
          slotDocumentSignature(listOptions, listKind, selectedIds) !==
            slotDocumentSignature(fresh, listKind, selectedIds);
        drift =
          metadataChanged ||
          !sameStringSet(
            listOptions ? documentActiveIds(listOptions) : [],
            documentActiveIds(fresh),
          );
      } else {
        // Compare only the cohort being exported — an unrelated cohort's cutoff
        // or roster change must not block this one (#1565 review pass 5).
        drift =
          !listOptions ||
          pickupCohortSignature(listOptions, pickupCohort) !==
            pickupCohortSignature(fresh, pickupCohort);
      }
      if (drift) {
        printTarget?.close();
        setIsExporting(false);
        setOptionsError(null);
        // Stash the drift warning instead of setting it directly. setListOptions
        // installs fresh options, which re-derives activeInstanceIds -> request
        // and immediately reruns the preview effect; that effect opens with
        // setError(null), so a message set here would be wiped by the very
        // refresh this branch triggers, flashing for a frame or never showing at
        // all. Stash it exactly like the 409 path so the refreshed preview
        // reapplies it once the fresh content lands (#1565 review pass 13).
        //
        // Commit under the shared generation: install `fresh` only when no newer
        // /options request has committed since this preflight started, then
        // advance the marker so an older revalidation resolving later is dropped
        // rather than overwriting this snapshot. If a newer request already won,
        // keep its options — but then setListOptions is skipped, so the else branch
        // must force a preview refetch or the stashed warning never surfaces
        // (#1565 review pass 1 P2). The stash is set first so it is live before
        // whichever branch triggers the preview rerun that re-applies it.
        pendingPreviewWarning.current =
          target === "slots"
            ? "Die Angebote für diesen Tag haben sich seit dem Laden geändert. Bitte prüfen und erneut exportieren."
            : "Die Abholzeiten für diesen Tag haben sich seit dem Laden geändert. Bitte prüfen und erneut exportieren.";
        if (exportOptionsGeneration > optionsCommittedGenerationRef.current) {
          optionsCommittedGenerationRef.current = exportOptionsGeneration;
          setListOptions(fresh);
        } else {
          // A newer /options refresh already committed a higher generation, so
          // installing `fresh` here would roll listOptions back and is correctly
          // skipped. But then NO preview dependency changes, so the preview effect
          // never reruns and the stashed warning above is never surfaced — the
          // export would stop with no file AND no explanation. Force a preview
          // refetch so the effect reruns and re-applies pendingPreviewWarning.current
          // once it lands. previewNonce is not a document input, so the separate
          // effect that clears the stash on a real document change does not fire
          // (#1565 review pass 1 P2 follow-up).
          setPreviewNonce((n) => n + 1);
        }
        return;
      }

      // No drift IN THIS DOCUMENT, but `fresh` may still differ from the installed
      // options OUTSIDE it (the drift check above is scoped to the exported list).
      // Commit this preflight's generation (guarded against a newer request that
      // already won) — the drift branch above only advances the marker when it
      // installs. Without committing here, this preflight's /options read (the
      // newest started) leaves the shared marker behind an older focus/visibility
      // revalidation that started earlier but resolves later; that stale sibling
      // would then pass its own `generation <= committed` check and overwrite the
      // snapshot, rolling active slot IDs or cutoff metadata back and re-firing a
      // stale preview / repeated export refusal until the next refresh (#1565 review
      // pass 2 P2). STAGE `fresh` alongside the commit so listOptions is brought up
      // to the newest snapshot after this attempt runs — advancing the marker while
      // dropping `fresh` would strand listOptions stale for out-of-document changes
      // (#1565 review pass 2 P2 follow-up).
      if (exportOptionsGeneration > optionsCommittedGenerationRef.current) {
        optionsCommittedGenerationRef.current = exportOptionsGeneration;
        refreshedOptions = fresh;
      }

      // The options guard above only sees slot/cohort METADATA. Roster membership
      // and attendance can drift without touching it — a child swaps groups or
      // cohorts, checks in/out, or is marked sick — and the export re-derives from
      // current data, so it could hand out different names or statuses than the
      // reviewed preview. Re-fetch the preview and compare its full content to
      // what the user reviewed; on any difference swap in the fresh list and ask
      // them to re-check rather than export something they never saw (#1565 review
      // pass 2).
      let freshPreview: SlotListResult;
      try {
        freshPreview = await fetchSlotListPreview(request);
      } catch (err: unknown) {
        printTarget?.close();
        setIsExporting(false);
        // The no-drift path already advanced the committed marker to this
        // preflight's generation and staged `fresh`; install it before bailing so
        // listOptions is not stranded on the previous snapshot while the advanced
        // marker drops older in-flight refreshes (its own guard skips the install
        // if a newer refresh has since won) (#1565 review pass 2 P2).
        commitRefreshedOptions();
        logger.error("slot_list_preview_revalidate_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(
          "Die Liste konnte vor dem Export nicht überprüft werden. Bitte versuchen Sie es erneut.",
        );
        return;
      }
      // Same mid-flight guard as after the options read: if the selection moved on
      // while this preview round-trip was in flight, `freshPreview` describes the
      // OLD document. The setResult(freshPreview) install below would overwrite the
      // current selection's already-committed preview with named children outside
      // the filters shown in the controls. Gate BEFORE comparing or installing it,
      // not just before the export (#1565 review pass 7 / pass 10).
      if (requestRef.current !== request) {
        printTarget?.close();
        setIsExporting(false);
        // The user navigated mid-export. If they only switched list_kind / source /
        // filter (the date is unchanged, so the staged snapshot is still valid),
        // install it now: the request already moved on, so this cannot disrupt the
        // abandoned export, and it keeps the generation-committed listOptions from
        // stranding stale on the list the user just switched to (#1565 review pass 2
        // P2 follow-up). A date change is skipped by commitRefreshedOptions' guard.
        commitRefreshedOptions();
        setError(
          "Die Auswahl hat sich während des Exports geändert. Bitte prüfen und erneut exportieren.",
        );
        return;
      }
      // resultSignature only covers the label, counters and rows. The backend
      // signature additionally folds in the exported "Enthalten" header summary,
      // which can drift on its own — e.g. a rowless selected slot is cancelled in
      // this window, leaving rows/counters identical but the contained-slot
      // summary changed. Comparing the backend signatures too closes that
      // header-only window; otherwise the drifted freshPreview.signature would be
      // handed to the export unchecked (#1565 review pass 8).
      if (
        !result ||
        resultSignature(freshPreview) !== resultSignature(result) ||
        freshPreview.signature !== result.signature
      ) {
        printTarget?.close();
        setIsExporting(false);
        setResult(freshPreview);
        const driftWarning =
          "Die Liste hat sich seit dem Laden geändert. Bitte prüfen und erneut exportieren.";
        // Stash the warning BEFORE committing the refreshed options, exactly like
        // the 409 and options-drift paths. Reaching here means the identity gate
        // above passed, so the captured request is still current and
        // commitRefreshedOptions is GUARANTEED to install the staged `fresh` — a new
        // options object that re-derives activeInstanceIds -> request and reruns the
        // preview effect, which opens with setError(null). A message set only via
        // setError here would be wiped by that very refresh, flashing for a frame or
        // never painting. Stashing it lets the refreshed preview reapply it once the
        // fresh content lands. The direct setError below is a belt-and-suspenders
        // fallback for the (currently unreachable) case where no refire fires; if a
        // refire does fire it is wiped and re-applied from the ref, netting one
        // stable warning (#1565 review pass 14 P2).
        pendingPreviewWarning.current = driftWarning;
        // Same stranding guard as the other early-exit paths: the no-drift path
        // advanced the committed marker and staged `fresh`, so install it here too
        // rather than leaving listOptions behind the advanced marker (#1565 review
        // pass 2 P2).
        commitRefreshedOptions();
        setError(driftWarning);
        return;
      }

      try {
        // Pass the verified preview's backend signature. The client-side guards
        // above cannot close the last window: the export endpoint rebuilds the
        // list in its OWN request AFTER this check, so a check-in/out, roster or
        // status change between freshPreview and that rebuild would still hand
        // out a file the user never saw. The backend re-derives, recomputes the
        // signature and refuses with 409 on any mismatch — so the downloaded file
        // is guaranteed to match the approved preview (#1565 review pass 2).
        //
        // exportSlotList runs its OWN network round-trip, and the date/filter
        // controls stay interactive throughout it (disabling them would thread a
        // disabled prop through the shared kit — see the isExporting note above).
        // The last identity gate before this call therefore cannot cover the
        // export await itself: the user could change the selection while the file
        // is being built, and without a further check the file for the abandoned
        // selection would still be downloaded/printed. Hand exportSlotList a commit
        // guard it evaluates AFTER the response arrives but BEFORE the file is
        // handed out, so a changed selection aborts the handoff instead of leaking
        // a document the UI has already left (#1565 review pass 10).
        await exportSlotList(
          request,
          format,
          mode,
          printTarget,
          freshPreview.signature,
          () => requestRef.current === request,
        );
        // The file has been handed out, so the request it was built from can no
        // longer be disrupted by a resulting state change. Installing the staged
        // options snapshot happens in the `finally` below, which runs for this
        // success path and for every early-exit export outcome alike — see the note
        // there (#1565 review pass 2 P2 follow-up).
      } catch (err: unknown) {
        // The selection changed during the export round-trip, so exportSlotList
        // refused to hand out the now-stale file (and already closed the print
        // tab). Ask the user to re-check and export again — this is not a failure
        // to log (#1565 review pass 10).
        if (err instanceof SlotListExportSupersededError) {
          setError(
            "Die Auswahl hat sich während des Exports geändert. Bitte prüfen und erneut exportieren.",
          );
          return;
        }
        // 409 = the backend's atomic drift refusal (its rebuild no longer matches
        // the verified signature). Refresh the preview so the user sees the new
        // content and re-checks, rather than silently failing. exportSlotList
        // already closed the print tab on this rejection. Stash the warning so it
        // survives the preview effect's setError(null) — setting it directly here
        // would be wiped by the refetch the nonce bump triggers, flashing the
        // message for a frame or never showing it at all (#1565 review pass 8).
        if (err instanceof SlotListExportError && err.status === 409) {
          pendingPreviewWarning.current =
            "Die Liste hat sich seit dem Laden geändert. Bitte prüfen und erneut exportieren.";
          // The 409 means schedule metadata or a pickup cutoff drifted after the
          // preflight /options read but before the backend's export rebuild.
          // Bumping only previewNonce refreshes the preview while request and the
          // controls keep deriving from the stale listOptions snapshot: a
          // since-cancelled slot stays selected and its explicit ID is re-sent to
          // the refreshed preview, leaving the UI inconsistent and forcing another
          // failed export before options finally update. Revalidate the options
          // snapshot alongside so activeInstanceIds is re-derived and the stale
          // selection is pruned (#1565 review pass 9).
          revalidateOptions();
          setPreviewNonce((n) => n + 1);
          return;
        }
        logger.error("slot_list_export_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(
          "Die Liste konnte nicht exportiert werden. Bitte versuchen Sie es erneut.",
        );
      } finally {
        setIsExporting(false);
        // Install the options snapshot the no-drift preflight staged, on EVERY
        // terminal outcome of the export round-trip — a clean success, a
        // superseded/aborted handoff (SlotListExportSupersededError), a 409 drift
        // refusal, or a generic failure. The no-drift path advanced the shared
        // committed-generation marker to exportOptionsGeneration when it staged
        // `fresh`; returning here without installing would strand listOptions on the
        // previous snapshot while that advanced marker drops older in-flight
        // refreshes, so a slot cancelled in a DIFFERENT list_kind (which the
        // document-scoped drift check correctly ignored) would stay selectable and
        // re-send its stale active IDs until the next focus refresh.
        // commitRefreshedOptions is a no-op once the snapshot is installed or was
        // never staged, and its own guard skips the install if a newer refresh has
        // since won (#1565 review pass 1 P2 / pass 2 P2).
        commitRefreshedOptions();
      }
    },
    [
      request,
      awaitingActiveSlots,
      filterLinkInvalid,
      target,
      dateISO,
      isManualSlotSelection,
      selectedSlotIds,
      listKind,
      pickupCohort,
      listOptions,
      result,
      revalidateOptions,
    ],
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
  // fully show. Those dropdowns only render when the preview exposes more than
  // one option (`result.groups`/`result.classes` are derived from the day's
  // rows), and they can only display/toggle values that ARE options.
  //
  // A bookmarked filter can point at a group/class with no rows for the selected
  // date + source: the reconciliation effect deliberately keeps that selection
  // (dropping it would broaden a confidential single-group list to the whole
  // cohort). Two shapes leave a value unreachable from the dropdown:
  //   - Dropdown hidden (zero or one visible option): the control disappears
  //     entirely, so the whole restriction is invisible.
  //   - Dropdown shown but the selection also contains a value that is NOT among
  //     the visible options (a stale/rowless id). The dropdown renders the
  //     visible members but cannot show or uncheck the hidden one, and
  //     nextSelection deliberately preserves unknown selections — so the hidden
  //     restriction silently narrows the list with no way to clear it there.
  // Render a removable chip for BOTH cases so the still-active filter stays
  // visible and clearable; removing it resets the whole restriction to "all"
  // (#1565 review pass 2 / pass 10).
  const hiddenActiveFilters: ActiveFilter[] = useMemo(() => {
    const chips: ActiveFilter[] = [];
    const groupDropdownShown = !!result && result.groups.length > 1;
    const classDropdownShown = !!result && result.classes.length > 1;
    const groupOptionIds = new Set(result?.groups.map((g) => g.id) ?? []);
    const classOptions = new Set(result?.classes ?? []);
    const groupHasHidden =
      selectedGroupIds !== null &&
      selectedGroupIds.some((id) => !groupOptionIds.has(id));
    const classHasHidden =
      selectedClasses !== null &&
      selectedClasses.some((c) => !classOptions.has(c));
    if (selectedGroupIds !== null && (!groupDropdownShown || groupHasHidden)) {
      chips.push({
        id: "group-restriction",
        label: "Gruppenfilter aktiv",
        onRemove: () => {
          setSelectedGroupIds(null);
          replaceListUrl({ selectedGroupIds: null });
        },
      });
    }
    if (selectedClasses !== null && (!classDropdownShown || classHasHidden)) {
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
    <div className="-mt-1.5 w-full">
      {/* Reached from Datenverwaltung → Exporte (no sidebar entry of its own). */}
      <BackButton referrer="/database/exports" />
      {/* Der Seitentitel steht auf dem Desktop in der Breadcrumb der
          Kopfzeile; PageHeaderWithSearch blendet seine Überschrift ab md aus. */}
      <PageHeaderWithSearch title="Tageslisten" />

      {/* Selection: source + date + data mode */}
      <section
        aria-label="Listenauswahl"
        className="moto-content-surface rounded-2xl border border-gray-200 bg-white p-4 shadow-sm sm:p-6"
      >
        <p className="text-moto-blue mb-2 text-xs font-semibold tracking-wide uppercase">
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
                    ? "border-moto-green/50 bg-moto-green/10"
                    : "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50"
                }`}
              >
                <span
                  className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-gray-100 ${
                    selected
                      ? "bg-moto-green/20 text-moto-green-strong"
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
              <span className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
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
                className={`text-moto-blue-strong hover:bg-moto-blue/8 rounded-md px-2 py-1 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
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
              <div className="mt-2">
                <Alert
                  type="info"
                  message="Abgesagte Angebote werden angezeigt, aber nicht in Tageslisten aufgenommen."
                />
              </div>
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
        <div className="mt-4">
          <Alert type="error" message={error} />
        </div>
      ) : null}

      {/* Preview + export: die Exporte stehen in der Titelzeile der Karte,
          nicht in einer eigenen Button-Zeile. */}
      <SectionCard
        title="Vorschau und Export"
        className="mt-4"
        actions={
          <>
            <Button
              type="button"
              variant="primary"
              size="md"
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
              size="md"
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
              size="md"
              disabled={isExporting || isLoading || !result}
              onClick={() => void handleExport("xlsx", "download")}
              className="gap-2"
            >
              <FileSpreadsheet className="h-4 w-4" aria-hidden />
              Excel
            </Button>
          </>
        }
      >
        <div className="min-w-0 space-y-3 border-b border-gray-100 pb-4">
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
                <EmptyState
                  className="py-8"
                  title="Keine Kinder in dieser Liste"
                  description={
                    isPickupBased
                      ? "Keine Kinder mit passender Abholzeit an diesem Tag."
                      : listKind
                        ? `Keine Kinder für „${selectedListLabel}“ an diesem Tag.`
                        : `Keine Kinder für „${selectedListLabel}“ an diesem Tag. Wählen Sie oben ein oder mehrere Angebote.`
                  }
                />
              }
            />
          )}
        </div>
      </SectionCard>
    </div>
  );
}
