"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import {
  Check,
  Clock3,
  Clock4,
  Download,
  FileSpreadsheet,
  Printer,
  Rows3,
  type LucideIcon,
} from "lucide-react";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DatePicker } from "~/components/ui/date-picker";
import { Loading } from "~/components/ui/loading";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { DesktopFilters } from "~/components/ui/page-header/DesktopFilters";
import type { FilterConfig } from "~/components/ui/page-header";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  formatDate,
  parseISODate,
  toISODate,
  todayISO,
} from "~/lib/date-helpers";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  exportSlotList,
  fetchSlotListOptions,
  fetchSlotListPreview,
  groupByOptionsFor,
  SLOT_LIST_GROUP_BY_LABELS,
  SLOT_LIST_SOURCE_LABELS,
  type SlotListFormat,
  type SlotListGroupBy,
  type SlotListOptionsResult,
  type SlotListPickupCohort,
  type SlotListResult,
  type SlotListRow,
  type SlotListSource,
  type SlotListTarget,
} from "~/lib/slot-lists-api";

const logger = createLogger({ component: "SlotListsPage" });

interface SelectionOption {
  id: "slots" | SlotListPickupCohort;
  target: SlotListTarget;
  pickupCohort?: SlotListPickupCohort;
  label: string;
  description: string;
  icon: LucideIcon;
}

// One short, single-line description each so every card is the same height.
const SELECTION_OPTIONS: SelectionOption[] = [
  {
    id: "slots",
    target: "slots",
    label: "Freie Angebotsauswahl",
    description: "Angebote des Tages auswählen",
    icon: Rows3,
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
];

const SOURCES: SlotListSource[] = ["planned", "actual", "reconciliation"];
const CANCELLED_SLOT_STATUS = "cancelled";
const ISO_DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

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
  short_day: "ganztag-1430",
  long_day: "ganztag-1600",
};

const URL_TO_SELECTION: Record<string, SelectionOption["id"]> = {
  angebote: "slots",
  "ganztag-1430": "short_day",
  "ganztag-1600": "long_day",
};

interface QueryParamsLike {
  get(name: string): string | null;
}

interface InitialListState {
  target: SlotListTarget;
  pickupCohort: SlotListPickupCohort;
  source: SlotListSource;
  groupBy: SlotListGroupBy;
  dateISO: string;
  selectedSlotIds: number[] | null;
  selectedGroupIds: number[] | null;
  selectedClasses: string[] | null;
}

function statusColor(row: SlotListRow): string {
  if (row.unplanned) return LOCATION_COLORS.SCHOOLYARD;
  if (row.present) return LOCATION_COLORS.GROUP_ROOM;
  // Justified (registered) absence — purple, visibly apart from an
  // unexplained "Fehlt" (red).
  if (row.excused) return LOCATION_COLORS.EXCUSED;
  if (row.planned) return LOCATION_COLORS.HOME;
  return LOCATION_COLORS.OTHER_ROOM;
}

function StatusBadge({ row }: Readonly<{ row: SlotListRow }>) {
  const color = statusColor(row);
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

interface CounterChip {
  label: string;
  value: number;
  color: string;
}

function buildCounterChips(result: SlotListResult): CounterChip[] {
  switch (result.source) {
    case "planned":
      return [
        {
          label: "Geplant",
          value: result.counters.planned,
          color: LOCATION_COLORS.OTHER_ROOM,
        },
      ];
    case "actual":
      return [
        {
          label: "Anwesend",
          value: result.counters.present,
          color: LOCATION_COLORS.GROUP_ROOM,
        },
      ];
    default:
      return [
        {
          label: "Geplant",
          value: result.counters.planned,
          color: LOCATION_COLORS.OTHER_ROOM,
        },
        {
          label: "Anwesend",
          value: result.counters.present,
          color: LOCATION_COLORS.GROUP_ROOM,
        },
        {
          label: "Fehlt",
          value: result.counters.missing,
          color: LOCATION_COLORS.HOME,
        },
        {
          label: "Abgemeldet",
          value: result.counters.excused,
          color: LOCATION_COLORS.EXCUSED,
        },
        {
          label: "Ungeplant",
          value: result.counters.unplanned,
          color: LOCATION_COLORS.SCHOOLYARD,
        },
      ];
  }
}

// nextStringSelection collapses a multi-select to null ("all") when the chosen
// set is empty or equal to the full option set; otherwise keeps the subset.
function nextSelection<T>(values: T[], total: number): T[] | null {
  return values.length === 0 || values.length === total ? null : values;
}

function parseNumberList(
  value: string | null,
  emptyToken: string | null = null,
): number[] | null {
  if (!value) return null;
  if (emptyToken && value === emptyToken) return [];
  const numbers = value
    .split(",")
    .map((part) => Number(part.trim()))
    .filter((id) => Number.isSafeInteger(id) && id > 0);
  return numbers.length > 0 ? numbers : null;
}

function parseStringList(value: string | null): string[] | null {
  if (!value) return null;
  const values = value
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
  return values.length > 0 ? values : null;
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
  const source = URL_TO_SOURCE[searchParams.get("basis") ?? ""] ?? "planned";
  const rawGroupBy =
    URL_TO_GROUP_BY[searchParams.get("gruppieren") ?? ""] ?? "";

  const rawDate = searchParams.get("datum");
  const dateISO =
    rawDate && ISO_DATE_PATTERN.test(rawDate) ? rawDate : todayISO();

  return {
    target,
    pickupCohort,
    source,
    groupBy: groupByOptionsFor(target).includes(rawGroupBy) ? rawGroupBy : "",
    dateISO,
    selectedSlotIds: parseNumberList(searchParams.get("angebote"), "keine"),
    selectedGroupIds: parseNumberList(searchParams.get("gruppen")),
    selectedClasses: parseStringList(searchParams.get("klassen")),
  };
}

function serializeListState(state: InitialListState): string {
  const params = new URLSearchParams();
  params.set("datum", state.dateISO);

  const selectionId = state.target === "slots" ? "slots" : state.pickupCohort;
  params.set("quelle", SELECTION_TO_URL[selectionId]);
  params.set("basis", SOURCE_TO_URL[state.source]);

  if (state.groupBy) {
    params.set("gruppieren", GROUP_BY_TO_URL[state.groupBy]);
  }
  if (state.target === "slots" && state.selectedSlotIds !== null) {
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
    params.set("klassen", state.selectedClasses.join(","));
  }

  return params.toString();
}

function offerCountLabel(count: number): string {
  return `${count} Angebot${count === 1 ? "" : "e"}`;
}

export default function SlotListsPage() {
  const { status: authStatus } = useSession({ required: true });
  const searchParams = useSearchParams();
  const router = useTenantRouter();
  const initialState = useMemo(
    () => parseInitialListState(searchParams),
    [searchParams],
  );
  const [target, setTarget] = useState<SlotListTarget>(initialState.target);
  const [pickupCohort, setPickupCohort] = useState<SlotListPickupCohort>(
    initialState.pickupCohort,
  );
  const [source, setSource] = useState<SlotListSource>(initialState.source);
  const [groupBy, setGroupBy] = useState<SlotListGroupBy>(initialState.groupBy);
  const [dateISO, setDateISO] = useState<string>(initialState.dateISO);
  // null = no restriction on that dimension; otherwise an explicit subset.
  const [selectedSlotIds, setSelectedSlotIds] = useState<number[] | null>(
    initialState.selectedSlotIds,
  );
  const [selectedGroupIds, setSelectedGroupIds] = useState<number[] | null>(
    initialState.selectedGroupIds,
  );
  const [selectedClasses, setSelectedClasses] = useState<string[] | null>(
    initialState.selectedClasses,
  );
  const [listOptions, setListOptions] = useState<SlotListOptionsResult | null>(
    null,
  );
  const [result, setResult] = useState<SlotListResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isOptionsLoading, setIsOptionsLoading] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isPickupBased = target === "pickup_cohort";

  const request = useMemo(
    () => ({
      date: dateISO,
      target,
      ...(isPickupBased ? { pickup_cohort: pickupCohort } : {}),
      source,
      ...(groupBy ? { group_by: groupBy } : {}),
      ...(!isPickupBased && selectedSlotIds !== null
        ? { instance_ids: selectedSlotIds }
        : {}),
      ...(selectedGroupIds ? { group_ids: selectedGroupIds } : {}),
      ...(selectedClasses ? { classes: selectedClasses } : {}),
    }),
    [
      dateISO,
      target,
      pickupCohort,
      isPickupBased,
      source,
      groupBy,
      selectedSlotIds,
      selectedGroupIds,
      selectedClasses,
    ],
  );
  const replaceListUrl = useCallback(
    (next: Partial<InitialListState>) => {
      const query = serializeListState({
        target,
        pickupCohort,
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
    setSource(next.source);
    setGroupBy(next.groupBy);
    setDateISO(next.dateISO);
    setSelectedSlotIds(next.selectedSlotIds);
    setSelectedGroupIds(next.selectedGroupIds);
    setSelectedClasses(next.selectedClasses);
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
      if (option.pickupCohort) setPickupCohort(option.pickupCohort);
      // slot/room/pickup_time groupings are target-specific, so a switch can
      // invalidate the current choice — reset to a flat list.
      setGroupBy("");
      resetFilters();
      replaceListUrl({
        target: option.target,
        pickupCohort: nextPickupCohort,
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
    (slotId: number) => {
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
    if (authStatus !== "authenticated") return;
    let cancelled = false;
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
      })
      .finally(() => {
        if (!cancelled) setIsOptionsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authStatus, dateISO]);

  useEffect(() => {
    if (authStatus !== "authenticated") return;
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
        setError("Die Vorschau konnte nicht geladen werden.");
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authStatus, request]);

  const handleExport = useCallback(
    async (format: SlotListFormat, mode: "download" | "print") => {
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
    [request],
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
      const active = (selectedGroupIds ?? groupIds).map(String);
      configs.push({
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        multiSelect: true,
        value: active,
        onChange: (v) => {
          const nextGroupIds = nextSelection(
            (v as string[]).map(Number),
            groupIds.length,
          );
          setSelectedGroupIds(nextGroupIds);
          replaceListUrl({ selectedGroupIds: nextGroupIds });
        },
        options: result.groups.map((g) => ({
          value: String(g.id),
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
          const nextClasses = nextSelection(
            v as string[],
            result.classes.length,
          );
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
      render: (r) => <StatusBadge row={r} />,
      sortValue: (r) => r.status_label,
    });
    return cols;
  }, [isPickupBased]);

  if (authStatus === "loading") {
    return <Loading fullPage={false} />;
  }

  const selectedOption = SELECTION_OPTIONS.find(
    (option) =>
      option.target === target &&
      (option.target === "slots" || option.pickupCohort === pickupCohort),
  );
  const selectedListLabel =
    result?.list_label ??
    (isPickupBased ? pickupOptionByCohort.get(pickupCohort)?.label : null) ??
    selectedOption?.label ??
    "";
  const includedMeta = result
    ? result.target === "pickup_cohort"
      ? result.list_label
      : selectedSlotIds === null
        ? offerCountLabel(selectableSlotIds.length)
        : offerCountLabel(selectedSlotIds.length)
    : "";
  const sourceMeta = result
    ? `${SLOT_LIST_SOURCE_LABELS[result.source]} – ${result.provenance}`
    : "";
  const groupByMeta = groupBy
    ? SLOT_LIST_GROUP_BY_LABELS[groupBy]
    : "Nicht gruppiert";

  return (
    <main className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6 lg:px-8">
      <header className="mb-6">
        <p className="text-sm font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
          Listen &amp; Exporte
        </p>
        <h1 className="mt-2 text-3xl font-semibold tracking-normal text-gray-950 sm:text-4xl">
          Tageslisten
        </h1>
        <p className="mt-2 max-w-3xl text-base leading-7 text-gray-600">
          Plan, Anwesenheit und Abgleich für Angebote und Ganztag prüfen, dann
          drucken oder exportieren. Die Notfallliste bleibt davon unberührt.
        </p>
      </header>

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
              (option.target === "slots" ||
                option.pickupCohort === pickupCohort);
            const availability = option.pickupCohort
              ? pickupOptionByCohort.get(option.pickupCohort)
              : null;
            const label = availability?.label ?? option.label;
            const hasAvailability = Boolean(availability);
            const slotCount = listOptions?.slots.length ?? 0;
            const available =
              option.target === "slots"
                ? slotCount > 0
                : (availability?.available ?? true);
            const detail =
              option.target === "slots"
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
                aria-pressed={selected}
                className={`flex h-full items-center gap-3 rounded-xl border p-3 text-left transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                  selected
                    ? "border-[#83CD2D]/50 bg-[#83CD2D]/10"
                    : available || !hasAvailability
                      ? "border-gray-200 bg-white hover:border-gray-300 hover:bg-gray-50"
                      : "border-gray-200 bg-gray-50 text-gray-500 hover:border-gray-300"
                }`}
              >
                <span
                  className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-xl ${
                    selected
                      ? "bg-[#83CD2D]/20 text-[#3F6F12]"
                      : available || !hasAvailability
                        ? "bg-gray-100 text-gray-500"
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

        {target === "slots" ? (
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
                replaceListUrl({ source: nextSource });
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
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex flex-wrap items-center gap-2">
            {result
              ? buildCounterChips(result).map((chip) => (
                  <span
                    key={chip.label}
                    className="inline-flex items-center gap-1.5 rounded-full bg-gray-50 px-3 py-1 text-sm font-medium"
                  >
                    <span
                      aria-hidden
                      className="inline-block h-2 w-2 rounded-full"
                      style={{ backgroundColor: chip.color }}
                    />
                    <span style={{ color: chip.color }}>
                      {chip.label}: {chip.value}
                    </span>
                  </span>
                ))
              : null}
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="primary"
              size="sm"
              isLoading={isExporting}
              loadingText="Erstelle PDF…"
              disabled={isLoading || !result}
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

        {result ? (
          <dl className="mt-3 flex flex-wrap gap-x-5 gap-y-1 border-t border-gray-100 pt-3 text-sm text-gray-600">
            <div className="flex gap-1.5">
              <dt className="font-medium text-gray-500">Datum:</dt>
              <dd className="font-medium text-gray-900">
                {formatDate(result.date)}
              </dd>
            </div>
            <div className="flex min-w-0 gap-1.5">
              <dt className="shrink-0 font-medium text-gray-500">
                Datenbasis:
              </dt>
              <dd className="min-w-0 truncate font-medium text-gray-900">
                {sourceMeta}
              </dd>
            </div>
            <div className="flex gap-1.5">
              <dt className="font-medium text-gray-500">Enthalten:</dt>
              <dd className="font-medium text-gray-900">{includedMeta}</dd>
            </div>
            <div className="flex gap-1.5">
              <dt className="font-medium text-gray-500">Gruppiert nach:</dt>
              <dd className="font-medium text-gray-900">{groupByMeta}</dd>
            </div>
          </dl>
        ) : null}

        {/* Standard filters that narrow the list (offering / group / class) */}
        {filterConfigs.length > 0 ? (
          <div className="mt-3">
            <DesktopFilters filters={filterConfigs} />
          </div>
        ) : null}

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
                    getRowKey={(r) => `${r.slot}-${r.student_id}`}
                  />
                </div>
              ))}
            </div>
          ) : (
            <DataTable
              columns={columns}
              rows={result?.rows ?? []}
              getRowKey={(r) => `${r.slot}-${r.student_id}`}
              isLoading={isLoading}
              emptyState={
                <div className="py-8 text-center text-gray-500">
                  {isPickupBased
                    ? "Keine Kinder mit passender Abholzeit an diesem Tag."
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
