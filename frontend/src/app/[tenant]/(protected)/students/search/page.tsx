"use client";

import {
  useState,
  useEffect,
  useRef,
  Suspense,
  useMemo,
  useCallback,
} from "react";
import { Download } from "lucide-react";
// SSE is handled globally by TenantAuthWrapper - real-time updates work automatically
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
  FilterSection,
} from "~/components/ui/page-header/types";
import { DetailIcons } from "~/components/ui/detail-modal-components";
import { studentService, groupService, roomService } from "~/lib/api";
import type { Student, Group, Room } from "~/lib/api";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import {
  berlinTodayISO,
  formatDate,
  isValidISODate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import {
  LOCATION_STATUSES,
  isHomeLocation,
  isPresentLocation,
  isSchoolyardLocation,
  isTransitLocation,
} from "~/lib/location-helper";
import {
  SCHOOL_YEAR_FILTER_OPTIONS,
  getSchoolYear,
} from "~/lib/student-helpers";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  StudentCard,
  SchoolClassIcon,
  GroupIcon,
  StudentInfoRow,
  PickupTimeRow,
  ArrivalTimeRow,
  StudentAbsenceRow,
  StudentPendingExcusedRow,
} from "~/components/students/student-card";
import { StudentExportModal } from "~/components/students/student-export-modal";
import { StudentCardGridSkeleton } from "~/components/students/student-card-skeleton";
import { SchoolCheckinFab } from "~/components/students/school-checkin-fab";
import { SchoolCheckinModeMobile } from "~/components/students/school-checkin-mode-mobile";
import {
  deriveCheckinState,
  useSchoolCheckinMode,
} from "~/lib/hooks/use-school-checkin-mode";
import { usePresenceMode } from "~/lib/tenant-context";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { useSWRAuth, useImmutableSWR } from "~/lib/swr";
import { SEARCH_ROOMS_LIST_CACHE_KEY } from "~/lib/swr/room-derived-caches";
import {
  normalizeSearchStudentsGroupId,
  SEARCH_STUDENTS_KEY_PREFIX,
  searchStudentsGroupScope,
} from "~/lib/swr/search-students-key";
import { activeService } from "~/lib/active-api";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import { createLogger } from "~/lib/logger";
import {
  getStudentAbsence,
  getStudentTimeStatus,
  getTimeStatusSortRank,
} from "~/lib/student-time-status";
import {
  getDayPlanningNotComingLabel,
  getStudentPresenceBadgePlanning,
} from "~/lib/day-planning-helper";
import {
  matchesTrackingFilter,
  parseTrackingFilter,
  resolveTrackingFilterAfterLabelChange,
  trackingFilterChipLabel,
  type TrackingFilter,
} from "./tracking-filter";
import { StudentSearchPageSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "StudentSearchPage" });
const EMPTY_STRING_ARRAY: string[] = [];
const EMPTY_STUDENT_ARRAY: Student[] = [];

type StatusFilter =
  | "all"
  | "anwesend"
  | "abwesend"
  | "unterwegs"
  | "schulhof"
  | "krank"
  | "klassenfahrt"
  | "entschuldigt";
type BooleanFilter = "all" | "yes" | "no";
type PickupStatusKind = "self" | "pickedUp" | "other" | "none" | "redacted";
type PickupStatusFilter = "all" | "self" | "pickedUp" | "none";
type DayStatusFilter = "all" | "comes_today" | "not_coming_today";
type SortMode = "name" | "arrival" | "pickup";
type GroupMode =
  | "none"
  | "status"
  | "room"
  | "arrival"
  | "pickup"
  | "pickup-status"
  | "companions";

const STATUS_FILTER_OPTIONS: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "Alle" },
  { value: "anwesend", label: "Anwesend" },
  { value: "abwesend", label: "Abwesend" },
  { value: "krank", label: "Krank" },
  { value: "klassenfahrt", label: "Klassenfahrt" },
  { value: "entschuldigt", label: "Entschuldigt" },
  { value: "unterwegs", label: "Unterwegs" },
  { value: "schulhof", label: "Schulhof" },
];

const DAY_STATUS_FILTER_OPTIONS: Array<{
  value: DayStatusFilter;
  label: string;
}> = [
  { value: "all", label: "Alle Kinder" },
  { value: "comes_today", label: "Kommt heute" },
  { value: "not_coming_today", label: "Kommt heute nicht" },
];

// Same wire values evaluated for a non-today planning date (#1939) — the UI
// must not say "heute" when the selection refers to another day, so the label
// drops it ("Kommt" / "Kommt nicht") while staying the same Kommt/-nicht filter.
const DAY_STATUS_FILTER_OPTIONS_OTHER_DAY: Array<{
  value: DayStatusFilter;
  label: string;
}> = [
  { value: "all", label: "Alle Kinder" },
  { value: "comes_today", label: "Kommt" },
  { value: "not_coming_today", label: "Kommt nicht" },
];

// Planning-date URL param (#1939). Deliberately NOT part of
// FILTER_QUERY_PARAMS: an absolute date restored from localStorage days later
// would silently show a stale day, so the date only lives in the URL.
const DATE_QUERY_PARAM = "date";

function normalizeDateParam(value: string | null): string | null {
  return value && isValidISODate(value) ? value : null;
}

// Last selectable planning day: the Sunday closing the CURRENT calendar week.
// Mirrors the backend's maxPlanningDate (#1939) — the only window the scheduler
// guarantees materialized timetable data for regardless of a tenant's
// materialization weekday / weeks-ahead settings (a run always materializes the
// week AFTER the run day, never the current partial one). The backend rejects
// later dates with 400, so the picker never offers them.
function maxPlanningIso(todayIso: string): string {
  const day = parseISODate(todayIso);
  day.setDate(day.getDate() + ((7 - day.getDay()) % 7));
  return toISODate(day);
}

const SORT_OPTIONS: Array<{ value: SortMode; label: string }> = [
  { value: "name", label: "Name A-Z" },
  { value: "arrival", label: "Nächste Ankunft" },
  { value: "pickup", label: "Nächste Abholung" },
];

const GROUP_OPTIONS: Array<{ value: GroupMode; label: string }> = [
  { value: "none", label: "Liste" },
  { value: "status", label: "Nach Status" },
  { value: "room", label: "Nach Raum" },
  { value: "arrival", label: "Nach Ankunftszeit" },
  { value: "pickup", label: "Nach Gehzeit" },
  { value: "pickup-status", label: "Nach Abholregelung" },
  { value: "companions", label: "Nach Laufgemeinschaft" },
];

const FILTER_QUERY_PARAMS = [
  "year",
  "school_class",
  "group_id",
  "room_id",
  "room_name",
  "bus",
  "photo_consent",
  "pickup_status",
  "pickup_time",
  "arrival_time",
  "day_status",
  "status",
  "tracking",
  "sort",
  "view",
] as const;

type FilterQueryParam = (typeof FILTER_QUERY_PARAMS)[number];
type PersistedSearchFilters = Partial<Record<FilterQueryParam, string>>;
type SearchParamReader = Pick<URLSearchParams, "get" | "has">;

const STUDENT_SEARCH_FILTER_STORAGE_PREFIX = "student-search:last-filters";
const FULL_STUDENT_SEARCH_PAGE_SIZE = 1000;

const SCHOOL_YEAR_DROPDOWN_OPTIONS = SCHOOL_YEAR_FILTER_OPTIONS.map(
  (option) => ({
    value: option.value,
    label: option.value === "all" ? "Alle Stufen" : `Stufe ${option.label}`,
  }),
);

const STATUS_GROUP_ORDER = new Map([
  ["Anwesend", 0],
  ["Unterwegs", 1],
  ["Schulhof", 2],
  ["Krank", 3],
  ["Entschuldigt", 4],
  ["Abwesend", 5],
]);

const BOOLEAN_FILTER_VALUES: readonly BooleanFilter[] = ["all", "yes", "no"];
const PICKUP_STATUS_FILTER_VALUES: readonly PickupStatusFilter[] = [
  "all",
  "self",
  "pickedUp",
  "none",
];

const BUS_FILTER_OPTIONS: Array<{ value: BooleanFilter; label: string }> = [
  { value: "all", label: "Alle Kinder" },
  { value: "yes", label: "Buskind" },
  { value: "no", label: "Kein Buskind" },
];

const PHOTO_CONSENT_FILTER_OPTIONS: Array<{
  value: BooleanFilter;
  label: string;
}> = [
  { value: "all", label: "Alle Kinder" },
  { value: "yes", label: "Fotoerlaubnis liegt vor" },
  { value: "no", label: "Keine Fotoerlaubnis" },
];

const PICKUP_STATUS_FILTER_OPTIONS: Array<{
  value: PickupStatusFilter;
  label: string;
}> = [
  { value: "all", label: "Alle Kinder" },
  { value: "self", label: "Geht alleine nach Hause" },
  { value: "pickedUp", label: "Wird abgeholt" },
  { value: "none", label: "Keine Abholregelung" },
];

// Status options split by what they read: the location-derived buckets only
// answer for today, the absence buckets come from the status days of whichever
// date is selected (#1939).
const LIVE_STATUS_FILTER_OPTIONS = [
  { value: "anwesend", label: "Anwesend" },
  { value: "abwesend", label: "Abwesend" },
  { value: "unterwegs", label: "Unterwegs" },
  { value: "schulhof", label: "Schulhof" },
] as const;

const PLANNED_ABSENCE_STATUS_FILTER_OPTIONS = [
  { value: "krank", label: "Krank" },
  { value: "klassenfahrt", label: "Klassenfahrt" },
  { value: "entschuldigt", label: "Entschuldigt" },
] as const;

function isPlannedAbsenceStatusFilter(filter: StatusFilter): boolean {
  return PLANNED_ABSENCE_STATUS_FILTER_OPTIONS.some(
    (option) => option.value === filter,
  );
}

const STATUS_FILTER_LABELS: Record<
  Exclude<StatusFilter, "all" | "anwesend">,
  string
> = {
  abwesend: "Abwesend",
  unterwegs: "Unterwegs",
  schulhof: "Schulhof",
  krank: "Krank",
  klassenfahrt: "Klassenfahrt",
  entschuldigt: "Entschuldigt",
};

function validQueryValue<T extends string>(
  value: string | null,
  validValues: readonly T[],
  fallback: T,
): T {
  return value && validValues.includes(value as T) ? (value as T) : fallback;
}

function searchParamsHavePersistedFilters(searchParams: SearchParamReader) {
  return FILTER_QUERY_PARAMS.some((key) => searchParams.has(key));
}

function persistedFiltersToSearchParams(filters: PersistedSearchFilters) {
  const params = new URLSearchParams();
  for (const key of FILTER_QUERY_PARAMS) {
    const value = filters[key];
    if (value) params.set(key, value);
  }
  return params;
}

function filtersFromSearchParams(
  searchParams: SearchParamReader,
): PersistedSearchFilters {
  const filters: PersistedSearchFilters = {};
  for (const key of FILTER_QUERY_PARAMS) {
    const value = searchParams.get(key);
    if (value) filters[key] = value;
  }
  return filters;
}

function normalizeStoredFilters(
  filters: PersistedSearchFilters,
): PersistedSearchFilters {
  const params = persistedFiltersToSearchParams(filters);
  const trackingParam = params.get("tracking");
  const trackingFilter =
    trackingParam && parseTrackingFilter(trackingParam).kind !== "invalid"
      ? trackingParam
      : "";

  return {
    year:
      validQueryValue(
        params.get("year"),
        SCHOOL_YEAR_DROPDOWN_OPTIONS.map((option) => option.value),
        "all",
      ) === "all"
        ? ""
        : (params.get("year") ?? ""),
    school_class: params.get("school_class") ?? "",
    group_id: normalizeSearchStudentsGroupId(params.get("group_id") ?? ""),
    room_id: params.get("room_id") ?? "",
    room_name: params.get("room_id") ? (params.get("room_name") ?? "") : "",
    bus:
      validQueryValue(params.get("bus"), BOOLEAN_FILTER_VALUES, "all") === "all"
        ? ""
        : (params.get("bus") ?? ""),
    photo_consent:
      validQueryValue(
        params.get("photo_consent"),
        BOOLEAN_FILTER_VALUES,
        "all",
      ) === "all"
        ? ""
        : (params.get("photo_consent") ?? ""),
    pickup_status:
      validQueryValue(
        params.get("pickup_status"),
        PICKUP_STATUS_FILTER_VALUES,
        "all",
      ) === "all"
        ? ""
        : (params.get("pickup_status") ?? ""),
    pickup_time: params.get("pickup_time") ?? "",
    arrival_time: params.get("arrival_time") ?? "",
    day_status:
      validQueryValue(
        params.get("day_status"),
        DAY_STATUS_FILTER_OPTIONS.map((option) => option.value),
        "all",
      ) === "all"
        ? ""
        : (params.get("day_status") ?? ""),
    status:
      validQueryValue(
        params.get("status"),
        STATUS_FILTER_OPTIONS.map((option) => option.value),
        "all",
      ) === "all"
        ? ""
        : (params.get("status") ?? ""),
    tracking: trackingFilter,
    sort:
      validQueryValue(
        params.get("sort"),
        SORT_OPTIONS.map((option) => option.value),
        "name",
      ) === "name"
        ? ""
        : (params.get("sort") ?? ""),
    view:
      validQueryValue(
        params.get("view"),
        GROUP_OPTIONS.map((option) => option.value),
        "none",
      ) === "none"
        ? ""
        : (params.get("view") ?? ""),
  };
}

function compactStoredFilters(filters: PersistedSearchFilters) {
  return Object.fromEntries(
    Object.entries(filters).filter(
      (entry): entry is [FilterQueryParam, string] => Boolean(entry[1]),
    ),
  ) as PersistedSearchFilters;
}

function safelyRemoveStoredFilters(storageKey: string) {
  try {
    window.localStorage.removeItem(storageKey);
  } catch {
    // Storage can be fully blocked by browser/privacy settings.
  }
}

function readStoredFilters(storageKey: string | null) {
  if (!storageKey || typeof window === "undefined") return null;

  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return null;
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      safelyRemoveStoredFilters(storageKey);
      return null;
    }
    return compactStoredFilters(
      normalizeStoredFilters(parsed as PersistedSearchFilters),
    );
  } catch {
    safelyRemoveStoredFilters(storageKey);
    return null;
  }
}

function writeStoredFilters(
  storageKey: string | null,
  filters: PersistedSearchFilters,
) {
  if (!storageKey || typeof window === "undefined") return;

  const compacted = compactStoredFilters(normalizeStoredFilters(filters));
  try {
    if (Object.keys(compacted).length === 0) {
      safelyRemoveStoredFilters(storageKey);
      return;
    }
    window.localStorage.setItem(storageKey, JSON.stringify(compacted));
  } catch {
    // localStorage can fail in private mode or when quota is exceeded. URL
    // persistence still works, so ignore storage failures.
  }
}

function removeStoredFilters(storageKey: string | null) {
  if (!storageKey || typeof window === "undefined") return;
  safelyRemoveStoredFilters(storageKey);
}

function buildSearchFilterStorageKey(
  user:
    | {
        id?: string | null;
        email?: string | null;
        tenantId?: number | null;
      }
    | undefined,
) {
  if (typeof window === "undefined") return null;

  const tenantKey =
    user?.tenantId !== undefined && user.tenantId !== null
      ? `tenant-${user.tenantId}`
      : `host-${window.location.host}`;
  const accountKey = user?.id ?? user?.email ?? "anonymous";
  return `${STUDENT_SEARCH_FILTER_STORAGE_PREFIX}:${tenantKey}:${accountKey}`;
}

function compareByName(a: Student, b: Student) {
  const lastCmp = (a.second_name ?? "").localeCompare(
    b.second_name ?? "",
    "de",
  );
  if (lastCmp !== 0) return lastCmp;
  return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
}

function statusLabelForStudent(student: Student): string {
  if (student.sick) return "Krank";
  if (student.class_trip) return "Klassenfahrt";
  if (student.excused) return "Entschuldigt";
  if (isSchoolyardLocation(student.current_location)) return "Schulhof";
  if (isTransitLocation(student.current_location)) return "Unterwegs";
  if (isHomeLocation(student.current_location)) return "Abwesend";
  return "Anwesend";
}

function roomLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";

  const location = student.current_location?.trim();
  if (!location || isHomeLocation(location)) return "Kein Raum";
  if (isTransitLocation(location)) return "Unterwegs";
  if (isSchoolyardLocation(location)) return "Schulhof";
  const separatorIndex = location.indexOf("-");
  if (separatorIndex >= 0) {
    const room = location.slice(separatorIndex + 1).trim();
    return room || "Kein Raum";
  }
  if (Object.values(LOCATION_STATUSES).some((status) => status === location)) {
    return "Kein Raum";
  }
  return location;
}

function pickupLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";
  return student.pickup_time ? `${student.pickup_time} Uhr` : "Keine Gehzeit";
}

function pickupStatusKind(student: Student): PickupStatusKind {
  if (student.has_full_access === false) return "redacted";

  const raw = student.pickup_status?.trim();
  if (!raw) return "none";

  const normalized = raw.toLowerCase();
  if (
    normalized.includes("alleine") ||
    normalized === "selbst" ||
    normalized === "self" ||
    normalized === "alone" ||
    normalized === "walk_home" ||
    normalized === "geht_alleine"
  ) {
    return "self";
  }
  if (
    normalized.includes("abgeholt") ||
    normalized === "parent" ||
    normalized === "parents" ||
    normalized === "guardian" ||
    normalized === "pickup" ||
    normalized === "picked_up" ||
    normalized === "wird_abgeholt"
  ) {
    return "pickedUp";
  }

  return "other";
}

function pickupStatusLabelForStudent(student: Student): string {
  const kind = pickupStatusKind(student);
  if (kind === "redacted") return "Nicht einsehbar";
  if (kind === "self") return "Geht alleine nach Hause";
  if (kind === "pickedUp") return "Wird abgeholt";
  if (kind === "none") return "Keine Abholregelung";
  return student.pickup_status?.trim() || "Keine Abholregelung";
}

function arrivalLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";
  return student.arrival_time
    ? `${student.arrival_time} Uhr`
    : "Keine Ankunftszeit";
}

const NO_COMPANION_GROUP_LABEL = "Ohne Laufgemeinschaft";
/**
 * A child whose links the caller may not see. The backend only resolves the
 * Laufgemeinschaft of full-access children (`collectFullAccessStudentIDs`), so
 * for a restricted child an empty list means "not readable", NOT "walks alone" —
 * with `gdpr.student_data_scope=group_supervisors_only` that is every child
 * outside the caller's own groups. Filing them under "Ohne Laufgemeinschaft"
 * would state as fact the one thing this page cannot know.
 */
const UNKNOWN_COMPANION_GROUP_LABEL = "Nicht einsehbar";

/**
 * Resolves the Laufgemeinschaft of every student on the page.
 *
 * The backend stores links as pairs, not groups: a Laufgemeinschaft is the set
 * of children reachable through those links on the shown day. So this walks the
 * links (union-find) and labels each resulting set with its members' names —
 * there is no group name to display, by design.
 *
 * A companion the filter or the pagination kept off the page still takes part
 * in the walk, it just cannot be NAMED (the list never received them — shipping
 * the names here would leak the children the caller filtered away). Two things
 * depend on that: a child whose only companion is hidden must not be presented
 * as walking alone, and a hidden child can be the bridge of an A-B-C group
 * whose visible members A and C would otherwise fall apart. Hidden members are
 * therefore counted into the label instead of being dropped.
 *
 * Each student is mapped to the component's identity (the union-find root) AND
 * its label. Bucketing has to happen on the identity: the label is built from
 * names, and two unrelated Laufgemeinschaften whose visible children happen to
 * be namesakes would otherwise be merged into one displayed group.
 */
interface CompanionGroup {
  /** Stable identity of the connected component — the bucket key. */
  readonly key: string;
  /** Display text; never used to decide what belongs together. */
  readonly label: string;
}

function companionGroupLabels(
  students: Student[],
): Map<string, CompanionGroup> {
  const parent = new Map<string, string>();
  const find = (id: string): string => {
    let root = id;
    while (parent.get(root) !== root) root = parent.get(root) ?? root;
    return root;
  };
  const union = (a: string, b: string) => {
    const rootA = find(a);
    const rootB = find(b);
    if (rootA !== rootB) parent.set(rootA, rootB);
  };
  const addNode = (id: string) => {
    if (!parent.has(id)) parent.set(id, id);
  };

  const onPage = new Set(students.map((student) => String(student.id)));
  for (const student of students) addNode(String(student.id));

  for (const student of students) {
    for (const companionId of student.companion_student_ids ?? []) {
      const companion = String(companionId);
      addNode(companion);
      union(String(student.id), companion);
    }
  }

  const members = new Map<string, Student[]>();
  const hiddenCounts = new Map<string, number>();
  for (const student of students) {
    const root = find(String(student.id));
    const bucket = members.get(root);
    if (bucket) bucket.push(student);
    else members.set(root, [student]);
  }
  for (const id of parent.keys()) {
    if (onPage.has(id)) continue;
    const root = find(id);
    hiddenCounts.set(root, (hiddenCounts.get(root) ?? 0) + 1);
  }

  const labels = new Map<string, CompanionGroup>();
  for (const [root, group] of members) {
    const hidden = hiddenCounts.get(root) ?? 0;
    // A set of one with nothing hidden behind it is a child that walks with
    // nobody on this day. With a hidden member it is a real Laufgemeinschaft
    // we can only show in part. The children who walk alone all share one
    // bucket, so that label IS their key; every real group keeps its own root.
    // A lone restricted child is the third case: nothing links to them on this
    // page and their own links were never sent, so "walks alone" is a guess.
    // Being pulled into a real group is still trustworthy — that link came
    // from a full-access child and is readable data.
    const lonely = group.length + hidden < 2;
    const redacted = lonely && group[0]?.has_full_access === false;
    const resolved: CompanionGroup = redacted
      ? {
          key: UNKNOWN_COMPANION_GROUP_LABEL,
          label: UNKNOWN_COMPANION_GROUP_LABEL,
        }
      : lonely
        ? { key: NO_COMPANION_GROUP_LABEL, label: NO_COMPANION_GROUP_LABEL }
        : {
            key: root,
            label: withHiddenMembers(
              group
                .map((student) => student.name)
                .sort((a, b) => a.localeCompare(b, "de"))
                .join(", "),
              hidden,
            ),
          };
    for (const student of group) labels.set(String(student.id), resolved);
  }
  return labels;
}

/**
 * Appends the count of group members that are not on the page, so a partly
 * filtered Laufgemeinschaft reads as incomplete instead of as the whole group.
 * Only the count travels — the names belong to children the caller filtered out.
 */
function withHiddenMembers(names: string, hidden: number): string {
  if (hidden < 1) return names;
  const suffix =
    hidden === 1 ? "1 weiteres Kind" : `${String(hidden)} weitere Kinder`;
  return `${names} (+ ${suffix} nicht angezeigt)`;
}

function groupStudents(students: Student[], groupMode: GroupMode) {
  if (groupMode === "none") return [];

  // Companion grouping is the one mode whose label depends on the OTHER
  // students on the page, so it is resolved once up front instead of per row.
  const companionLabels =
    groupMode === "companions" ? companionGroupLabels(students) : null;

  // Every mode but the companion one is grouped BY its label; companions are
  // grouped by component identity and only rendered with the label.
  const groups = new Map<string, { label: string; items: Student[] }>();
  for (const student of students) {
    const companionGroup = companionLabels?.get(String(student.id));
    const label = companionLabels
      ? (companionGroup?.label ?? NO_COMPANION_GROUP_LABEL)
      : groupMode === "status"
        ? statusLabelForStudent(student)
        : groupMode === "room"
          ? roomLabelForStudent(student)
          : groupMode === "arrival"
            ? arrivalLabelForStudent(student)
            : groupMode === "pickup"
              ? pickupLabelForStudent(student)
              : pickupStatusLabelForStudent(student);
    const key = companionLabels
      ? (companionGroup?.key ?? NO_COMPANION_GROUP_LABEL)
      : label;
    const bucket = groups.get(key);
    if (bucket) {
      bucket.items.push(student);
    } else {
      groups.set(key, { label, items: [student] });
    }
  }

  // The key travels with the group: two unrelated Laufgemeinschaften can render
  // the SAME label (namesake children, identical hidden-member count), and a
  // duplicate React key would let one section's rows reconcile into the other.
  return Array.from(groups.entries())
    .map(([key, { label, items }]) => ({
      key,
      label,
      items,
    }))
    .sort((a, b) => {
      if (groupMode === "status") {
        return (
          (STATUS_GROUP_ORDER.get(a.label) ?? 99) -
          (STATUS_GROUP_ORDER.get(b.label) ?? 99)
        );
      }
      if (groupMode === "pickup" || groupMode === "arrival") {
        const rank = (label: string) =>
          label === "Keine Gehzeit" || label === "Keine Ankunftszeit"
            ? "99"
            : label === "Nicht einsehbar"
              ? "zz"
              : label;
        return rank(a.label).localeCompare(rank(b.label), "de");
      }
      if (groupMode === "companions") {
        // Real Laufgemeinschaften first, then the children who walk with
        // nobody, and last the ones this account may not read.
        const rank = (label: string) =>
          label === NO_COMPANION_GROUP_LABEL
            ? 1
            : label === UNKNOWN_COMPANION_GROUP_LABEL
              ? 2
              : 0;
        const rankCmp = rank(a.label) - rank(b.label);
        if (rankCmp !== 0) return rankCmp;
      }
      if (groupMode === "pickup-status") {
        const rank = (label: string) =>
          label === "Geht alleine nach Hause"
            ? 0
            : label === "Wird abgeholt"
              ? 1
              : label === "Nicht einsehbar"
                ? 99
                : label === "Keine Abholregelung"
                  ? 98
                  : 2;
        const rankCmp = rank(a.label) - rank(b.label);
        if (rankCmp !== 0) return rankCmp;
      }
      return a.label.localeCompare(b.label, "de");
    });
}

function compareByPickupTime(a: Student, b: Student, now: Date) {
  const statusA = getStudentTimeStatus({
    plannedTime: a.pickup_time,
    actualTime: a.actual_pickup_time,
    now,
    sick: a.sick,
    classTrip: a.class_trip,
    excused: a.excused,
  });
  const statusB = getStudentTimeStatus({
    plannedTime: b.pickup_time,
    actualTime: b.actual_pickup_time,
    now,
    sick: b.sick,
    classTrip: b.class_trip,
    excused: b.excused,
  });
  const rankA = getTimeStatusSortRank(statusA);
  const rankB = getTimeStatusSortRank(statusB);
  if (rankA !== rankB) return rankA - rankB;

  const timeA = a.pickup_time;
  const timeB = b.pickup_time;
  if (timeA && !timeB) return -1;
  if (!timeA && timeB) return 1;
  if (timeA && timeB) {
    const timeCmp = timeA.localeCompare(timeB);
    if (timeCmp !== 0) return timeCmp;
  }
  return compareByName(a, b);
}

function orderedRooms(
  rooms: Room[],
  groupRoomNames: string[],
  supervisedRoomNames: string[],
) {
  const priorityNames = new Set(
    [...groupRoomNames, ...supervisedRoomNames].map((name) =>
      name.trim().toLowerCase(),
    ),
  );

  return [...rooms].sort((a, b) => {
    const aPriority = priorityNames.has(a.name.trim().toLowerCase());
    const bPriority = priorityNames.has(b.name.trim().toLowerCase());
    if (aPriority !== bPriority) return aPriority ? -1 : 1;
    return a.name.localeCompare(b.name, "de");
  });
}

function SearchPageContent() {
  const router = useTenantRouter();
  // Use required: true to auto-redirect unauthenticated users (same pattern as /active-supervisions)
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });
  const searchParams = useSearchParams();
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const storageKey = useMemo(
    () => buildSearchFilterStorageKey(session?.user),
    [session?.user],
  );
  const hasUrlFilterParams = searchParamsHavePersistedFilters(searchParams);
  const storedInitialFilters = useMemo(
    () => (hasUrlFilterParams ? null : readStoredFilters(storageKey)),
    [hasUrlFilterParams, storageKey],
  );
  const initialFilterParams = useMemo(
    () =>
      storedInitialFilters
        ? persistedFiltersToSearchParams(storedInitialFilters)
        : searchParams,
    [searchParams, storedInitialFilters],
  );

  // Read initial filters from URL params so refreshes, revisits via browser
  // history, and copied links restore the same operational view. When no URL
  // filters are present, fall back to the last browser-local PWA view.
  const initialAttendanceFilter = validQueryValue(
    initialFilterParams.get("status"),
    STATUS_FILTER_OPTIONS.map((option) => option.value),
    "all",
  );
  const initialDayStatusFilter = validQueryValue(
    initialFilterParams.get("day_status"),
    DAY_STATUS_FILTER_OPTIONS.map((option) => option.value),
    "all",
  );
  const initialYear = validQueryValue(
    initialFilterParams.get("year"),
    SCHOOL_YEAR_DROPDOWN_OPTIONS.map((option) => option.value),
    "all",
  );
  const initialTrackingParam = initialFilterParams.get("tracking") ?? "all";
  const initialTrackingFilter =
    parseTrackingFilter(initialTrackingParam).kind === "invalid"
      ? "all"
      : (initialTrackingParam as TrackingFilter);
  const initialSortMode = validQueryValue(
    initialFilterParams.get("sort"),
    SORT_OPTIONS.map((option) => option.value),
    "name",
  );
  const initialGroupMode = validQueryValue(
    initialFilterParams.get("view"),
    GROUP_OPTIONS.map((option) => option.value),
    "none",
  );

  // Room filter, populated when the user lands here from the room detail
  // page's "In Kindersuche öffnen" link (#1323). room_name is purely a
  // display affordance for the chip; the backend filter only uses room_id.
  const initialRoomId = initialFilterParams.get("room_id") ?? "";
  const initialRoomName = initialFilterParams.get("room_name") ?? "";

  // Search and filter state
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState(""); // Debounced version for SWR key
  const [selectedGroup, setSelectedGroup] = useState(
    normalizeSearchStudentsGroupId(initialFilterParams.get("group_id") ?? ""),
  );
  const [selectedSchoolClass, setSelectedSchoolClass] = useState(
    initialFilterParams.get("school_class") ?? "",
  );
  const [selectedYear, setSelectedYear] = useState<string>(initialYear);
  const [attendanceFilter, setAttendanceFilter] = useState<StatusFilter>(
    initialAttendanceFilter,
  );
  const [busFilter, setBusFilter] = useState<BooleanFilter>(
    validQueryValue(
      initialFilterParams.get("bus"),
      BOOLEAN_FILTER_VALUES,
      "all",
    ),
  );
  const [photoConsentFilter, setPhotoConsentFilter] = useState<BooleanFilter>(
    validQueryValue(
      initialFilterParams.get("photo_consent"),
      BOOLEAN_FILTER_VALUES,
      "all",
    ),
  );
  const [pickupStatusFilter, setPickupStatusFilter] =
    useState<PickupStatusFilter>(
      validQueryValue(
        initialFilterParams.get("pickup_status"),
        PICKUP_STATUS_FILTER_VALUES,
        "all",
      ),
    );
  const [dayStatusFilter, setDayStatusFilter] = useState<DayStatusFilter>(
    initialDayStatusFilter,
  );
  const [pickupTimeFilter, setPickupTimeFilter] = useState(
    initialFilterParams.get("pickup_time") ?? "all",
  );
  const [arrivalTimeFilter, setArrivalTimeFilter] = useState(
    initialFilterParams.get("arrival_time") ?? "all",
  );
  const [trackingFilter, setTrackingFilter] = useState<TrackingFilter>(
    initialTrackingFilter,
  );
  const [sortMode, setSortMode] = useState<SortMode>(initialSortMode);
  const [groupMode, setGroupMode] = useState<GroupMode>(initialGroupMode);
  const [selectedRoomId, setSelectedRoomId] = useState(initialRoomId);
  const [selectedRoomName, setSelectedRoomName] = useState(initialRoomName);
  const [isExportOpen, setIsExportOpen] = useState(false);

  // Planning date (#1939). Always read from the real URL, never from the
  // stored-filter fallback. null = "follow the school-local today": the
  // default view tracks the Berlin day as it advances past midnight, while an
  // explicitly chosen date stays fixed. Only a strictly-future URL date within
  // the planning horizon is kept explicit: a date naming today collapses to the
  // implicit default, a past date (bookmarked or hand-edited URL) is rejected
  // because planning is future-only, and a date beyond the horizon is rejected
  // because the backend would answer it with 400 (see maxPlanningIso).
  const [explicitDate, setExplicitDate] = useState<string | null>(() => {
    const fromUrl = normalizeDateParam(searchParams.get(DATE_QUERY_PARAM));
    const today = berlinTodayISO();
    return fromUrl && fromUrl > today && fromUrl <= maxPlanningIso(today)
      ? fromUrl
      : null;
  });

  const updateUrlParams = useCallback(
    (
      patch: Partial<
        Record<
          (typeof FILTER_QUERY_PARAMS)[number] | typeof DATE_QUERY_PARAM,
          string
        >
      >,
    ) => {
      if (typeof window === "undefined") return;
      const url = new URL(window.location.href);
      for (const [key, value] of Object.entries(patch)) {
        if (value === undefined || value === "") {
          url.searchParams.delete(key);
        } else {
          url.searchParams.set(key, value);
        }
      }
      window.history.replaceState(
        window.history.state ?? null,
        "",
        url.toString(),
      );
      writeStoredFilters(storageKey, filtersFromSearchParams(url.searchParams));
    },
    [storageKey],
  );

  const applyPersistedFilters = useCallback(
    (filters: PersistedSearchFilters) => {
      const params = persistedFiltersToSearchParams(
        compactStoredFilters(normalizeStoredFilters(filters)),
      );

      setSelectedGroup(
        normalizeSearchStudentsGroupId(params.get("group_id") ?? ""),
      );
      setSelectedSchoolClass(params.get("school_class") ?? "");
      setSelectedYear(
        validQueryValue(
          params.get("year"),
          SCHOOL_YEAR_DROPDOWN_OPTIONS.map((option) => option.value),
          "all",
        ),
      );
      setAttendanceFilter(
        validQueryValue(
          params.get("status"),
          STATUS_FILTER_OPTIONS.map((option) => option.value),
          "all",
        ),
      );
      setPickupTimeFilter(params.get("pickup_time") ?? "all");
      setArrivalTimeFilter(params.get("arrival_time") ?? "all");
      setBusFilter(
        validQueryValue(params.get("bus"), BOOLEAN_FILTER_VALUES, "all"),
      );
      setPhotoConsentFilter(
        validQueryValue(
          params.get("photo_consent"),
          BOOLEAN_FILTER_VALUES,
          "all",
        ),
      );
      setPickupStatusFilter(
        validQueryValue(
          params.get("pickup_status"),
          PICKUP_STATUS_FILTER_VALUES,
          "all",
        ),
      );
      setDayStatusFilter(
        validQueryValue(
          params.get("day_status"),
          DAY_STATUS_FILTER_OPTIONS.map((option) => option.value),
          "all",
        ),
      );

      const trackingParam = params.get("tracking") ?? "all";
      setTrackingFilter(
        parseTrackingFilter(trackingParam).kind === "invalid"
          ? "all"
          : (trackingParam as TrackingFilter),
      );

      setSortMode(
        validQueryValue(
          params.get("sort"),
          SORT_OPTIONS.map((option) => option.value),
          "name",
        ),
      );
      setGroupMode(
        validQueryValue(
          params.get("view"),
          GROUP_OPTIONS.map((option) => option.value),
          "none",
        ),
      );
      setSelectedRoomId(params.get("room_id") ?? "");
      setSelectedRoomName(params.get("room_name") ?? "");
    },
    [],
  );

  useEffect(() => {
    if (!storageKey) return;

    if (hasUrlFilterParams) {
      writeStoredFilters(storageKey, filtersFromSearchParams(searchParams));
      return;
    }

    const storedFilters = readStoredFilters(storageKey);
    if (!storedFilters || Object.keys(storedFilters).length === 0) return;

    applyPersistedFilters(storedFilters);
    updateUrlParams(storedFilters);
  }, [
    applyPersistedFilters,
    hasUrlFilterParams,
    searchParams,
    storageKey,
    updateUrlParams,
  ]);

  // Current time for pickup urgency calculation (updates every minute)
  const now = useMinuteClock();

  // Planning-date context (#1939). todayIso derives from the minute clock so
  // the "Heute" anchor rolls over at midnight without a reload — and it is
  // the SCHOOL-local (Berlin) day, matching the backend's default, not the
  // browser's timezone.
  const todayIso = berlinTodayISO(now);
  // An explicitly selected future date can age into the past if this tab stays
  // open across the selected day and the following midnight: todayIso rolls
  // forward on the minute clock while explicitDate stays frozen. Planning is
  // future-only, so once the stored date has fallen behind the Berlin day,
  // ignore it and follow "Heute" again — never send a past date to the backend
  // or render yesterday's planning. The effect below reconciles the persisted
  // state and URL so the picker stops showing the stale day too (#1939).
  const selectedDate =
    explicitDate && explicitDate >= todayIso ? explicitDate : todayIso;
  const isToday = selectedDate === todayIso;
  const tomorrowIso = useMemo(() => {
    const tomorrow = parseISODate(todayIso);
    tomorrow.setDate(tomorrow.getDate() + 1);
    return toISODate(tomorrow);
  }, [todayIso]);
  // The horizon end only ever moves forward as todayIso advances, so an
  // explicit date that was valid once can never age out at the far end — no
  // reconciliation effect needed beyond the past-date one below.
  const maxDateIso = useMemo(() => maxPlanningIso(todayIso), [todayIso]);
  // Urgency coloring and time-status sorting compare against "now". For a
  // non-today date every planned time is neutral, so compare against that
  // day's local midnight instead of the current clock.
  const planningNow = useMemo(
    () => (isToday ? now : parseISODate(selectedDate)),
    [isToday, now, selectedDate],
  );

  const updateSelectedDate = useCallback(
    (value: string) => {
      // Planning is future-only and horizon-bounded: never store a day the
      // backend rejects. The picker already disables out-of-range dates, so
      // this only guards a stray caller — clamp into [today, maxDateIso].
      const bounded = value > maxDateIso ? maxDateIso : value;
      const clamped = bounded < todayIso ? todayIso : bounded;
      // Picking the current Berlin day means "follow today", not a fixed date.
      // Compare against the same clock-derived todayIso the buttons pass, NOT a
      // fresh berlinTodayISO(): near Berlin midnight the minute clock can still
      // read yesterday for up to 60s, and mixing the two days would store
      // yesterday as an explicit date and pin the page there when the user taps
      // "Heute" (#1939).
      const explicit = clamped === todayIso ? null : clamped;
      setExplicitDate(explicit);
      updateUrlParams({ date: explicit ?? "" });
    },
    [todayIso, maxDateIso, updateUrlParams],
  );

  // The explicitDate initializer rejects a URL `date` that is malformed, past,
  // or names today (planning is future-only), collapsing it to null. When it
  // does, the raw query param is still sitting in the address bar, so a copied
  // link or the URL-state contract would keep advertising a date the page no
  // longer uses. Reconcile the URL once on mount to what was actually accepted.
  // The aging effect below only covers a date that ages out *after* mount
  // (explicitDate is non-null there); this covers the already-rejected case.
  useEffect(() => {
    const rawDate = searchParams.get(DATE_QUERY_PARAM);
    if (rawDate !== null && rawDate !== (explicitDate ?? "")) {
      updateUrlParams({ date: explicitDate ?? "" });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Once an explicitly selected date has aged past the Berlin day (tab left
  // open across midnight), drop it so the picker, URL, and stored filters stop
  // carrying a past planning date. selectedDate above already ignores it; this
  // reconciles the persisted state (#1939).
  useEffect(() => {
    if (explicitDate && explicitDate < todayIso) {
      setExplicitDate(null);
      updateUrlParams({ date: "" });
    }
  }, [explicitDate, todayIso, updateUrlParams]);

  const dayStatusFilterOptions = isToday
    ? DAY_STATUS_FILTER_OPTIONS
    : DAY_STATUS_FILTER_OPTIONS_OTHER_DAY;

  // Live presence data describes today. For any other planning date the
  // realtime filters are neutralized and hidden so current whereabouts are
  // never read as (or filtered like) a plan for that day. The absence statuses
  // are the exception: they come from the status days recorded for the selected
  // date, not from the live location, so planning by them is the whole point
  // ("wer ist am Freitag krank gemeldet") and they stay selectable (#1939).
  const effectiveAttendanceFilter: StatusFilter =
    isToday || isPlannedAbsenceStatusFilter(attendanceFilter)
      ? attendanceFilter
      : "all";
  const effectiveTrackingFilter: TrackingFilter = isToday
    ? trackingFilter
    : "all";
  const effectiveRoomId = isToday ? selectedRoomId : "";
  const effectiveGroupMode: GroupMode =
    isToday || (groupMode !== "status" && groupMode !== "room")
      ? groupMode
      : "none";

  // OGS group tracking via shared BFF endpoint with SWR caching
  // This eliminates 2 separate API calls with 2 auth() calls each
  const { userContext } = useUserContext();
  const myGroups = userContext?.educationalGroupIds ?? EMPTY_STRING_ARRAY;
  const myGroupRooms =
    userContext?.educationalGroupRoomNames ?? EMPTY_STRING_ARRAY;
  const mySupervisedRooms =
    userContext?.supervisedRoomNames ?? EMPTY_STRING_ARRAY;

  // Page-level school check-in/out mode. When active, clicking a card toggles
  // the student's attendance instead of navigating to the detail page.
  //
  // Only exposed in binary-mode tenants. Detailed-mode schools check
  // students in via the RFID kiosk and a parallel web button would create
  // confusing divergent state.
  const presenceMode = usePresenceMode();
  const isBinaryMode = presenceMode === "binary";
  const schoolCheckin = useSchoolCheckinMode();
  const {
    enabled: studentPhotosEnabled,
    isLoading: studentPhotosSettingLoading,
  } = useStudentPhotosEnabled();

  // Debounce search term for SWR key (prevents excessive API calls while typing)
  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      setDebouncedSearchTerm(searchTerm);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchTerm]);

  // Fetch groups with SWR (immutable - only fetched once)
  const { data: groups = [] } = useImmutableSWR<Group[]>(
    "search-groups-list",
    async () => {
      try {
        return await groupService.getGroups();
      } catch {
        // User might not have groups:read permission - continue with empty list
        logger.warn("could not load groups for filter");
        return [];
      }
    },
  );

  // Room names/options are mutable from the rooms admin page, so this must
  // revalidate stale cache entries when the search page remounts.
  const { data: rooms = [] } = useSWRAuth<Room[]>(
    SEARCH_ROOMS_LIST_CACHE_KEY,
    async () => {
      try {
        return await roomService.getRooms({ page: 1, pageSize: 1000 });
      } catch {
        logger.warn("could not load rooms for filter");
        return [];
      }
    },
  );

  const { data: schoolClassOptionsData } = useSWRAuth<unknown>(
    "search-student-school-classes",
    async () => studentService.getSchoolClasses(),
    { revalidateOnFocus: false },
  );
  const schoolClassOptions = useMemo(
    () =>
      Array.isArray(schoolClassOptionsData)
        ? schoolClassOptionsData.filter(
            (item): item is string => typeof item === "string",
          )
        : [],
    [schoolClassOptionsData],
  );

  // Generate SWR cache key for students (changes when filters change → SWR auto-cancels old requests)
  // Note: User context is only for badge styling, not for fetching students
  const photoConsentFeatureAvailable = studentPhotosSettingLoading
    ? true
    : studentPhotosEnabled;
  const photoConsentFeatureState = studentPhotosSettingLoading
    ? "photos-loading"
    : studentPhotosEnabled
      ? "photos-enabled"
      : "photos-disabled";
  const requestedPhotoConsentFilter = photoConsentFeatureAvailable
    ? photoConsentFilter
    : "all";
  // bus / photo_consent / pickup_status are now real backend filters (#1492),
  // applied server-side in the same in-memory pass as day_status, so the
  // backend returns correctly-filtered and correctly-counted pages. The cache
  // key just has to vary with every filter value so SWR refetches on change.
  // Companion links are fetched only for the Laufgemeinschaft grouping — they
  // cost an extra backend query, and no other view reads them. Part of the
  // cache key so switching into that grouping triggers exactly one refetch.
  // Sits BEFORE pickup_status on purpose: that filter must stay the key's last
  // segment (asserted by the #1492 cache-key test).
  const wantsCompanions = groupMode === "companions";
  // The key carries selectedDate AND the today/non-today mode: when an
  // explicitly selected future date becomes today at midnight, selectedDate
  // is unchanged but the request semantics flip (date omitted, live
  // attendance included) — without the mode in the key SWR would keep
  // serving the stale future-planning response under live controls (#1939).
  //
  // The group filter leads the key as a fixed `g{id}`/`gall` scope segment
  // (#2097) so the SSE flush can read it back and skip a refetch when a
  // check-in event names a group this view does not show. It has to sit BEFORE
  // the free-text term: the term may contain dashes ("Anna-Lena"), so no
  // segment after it can be located positionally. See lib/swr/search-students-key.
  const studentsCacheKey = `${SEARCH_STUDENTS_KEY_PREFIX}${searchStudentsGroupScope(selectedGroup)}-${debouncedSearchTerm}-${selectedSchoolClass}-${effectiveRoomId}-${dayStatusFilter}-${selectedDate}-${isToday ? "today" : "planning"}-${photoConsentFeatureState}-${busFilter}-${requestedPhotoConsentFilter}-${wantsCompanions}-${pickupStatusFilter}`;

  // Fetch students with SWR (automatic deduplication, cancellation, and revalidation)
  const {
    data: studentsData,
    isLoading: isSearching,
    error: studentsError,
  } = useSWRAuth<{
    students: Student[];
    requestDate: string;
    requestIsToday: boolean;
  }>(
    studentsCacheKey,
    async () => {
      const filters = {
        search: debouncedSearchTerm,
        groupId: selectedGroup,
        schoolClass: selectedSchoolClass || undefined,
        roomId: effectiveRoomId || undefined,
        dayStatus: dayStatusFilter === "all" ? undefined : dayStatusFilter,
        // Planning date (#1939): omitted for today so today's requests stay
        // byte-identical with the pre-date behavior.
        date: isToday ? undefined : selectedDate,
        // Administrative filters (#1492) are now applied server-side, so the
        // backend returns the correctly-filtered, correctly-counted page set
        // directly, no client-side full-page fetch + filter required.
        bus: busFilter === "all" ? undefined : busFilter,
        photoConsent:
          requestedPhotoConsentFilter === "all"
            ? undefined
            : requestedPhotoConsentFilter,
        pickupStatus:
          pickupStatusFilter === "all" ? undefined : pickupStatusFilter,
        // When filtering by room, this page is the "see all" target the
        // room-detail modal links to (#1374). The modal itself caps at
        // 200 cards and shows a truncation notice; this page must show
        // every occupant so the overflow link actually delivers.
        // 1000 covers any realistic combined-group / assembly-room
        // session well above what backend ParsePagination would return
        // by default (50). General search keeps the default.
        pageSize: effectiveRoomId ? FULL_STUDENT_SEARCH_PAGE_SIZE : undefined,
        includePickupTimes: true,
        includeArrivalTimes: true,
        includeCompanions: wantsCompanions,
        // Slim wire projection (#2097): this page requests up to 1000 rows in
        // one trip, and the full StudentResponse ships address, health info,
        // supervisor notes, guardian contacts, the weekday departure maps and
        // consent/row timestamps that no card, filter or grouping mode here
        // reads — the child detail page fetches the full record itself.
        // Measured 78% smaller for the same rows.
        view: "slim" as const,
      };

      const result = await studentService.getStudents(filters);
      // Tag the response with the date AND today/planning mode it was fetched
      // for so date-sensitive rendering can tell a fresh result apart from
      // keepPreviousData holding the previous request's rows. The mode matters
      // on top of the date: when a selected future date rolls over to today at
      // midnight, selectedDate is unchanged but the semantics flip from planning
      // to live — the stale planning rows must not be shown under live presence
      // badges and check-in controls (#1939).
      return {
        students: result.students,
        requestDate: selectedDate,
        requestIsToday: isToday,
      };
    },
    {
      // Keep previous data while fetching (prevents loading flash)
      keepPreviousData: true,
    },
  );

  // Keep room state and room_id/room_name query params in sync without a
  // Next.js navigation, which would discard SWR data and flash the loading
  // skeleton. Merge into the existing state object instead of replacing it.
  // App Router stashes routing metadata (scroll restoration, RSC cache keys)
  // on window.history.state, and clobbering it with `{}` can degrade browser
  // back/forward into a hard reload for this entry.
  const updateRoomFilter = useCallback(
    (roomId: string, roomName: string) => {
      setSelectedRoomId(roomId);
      setSelectedRoomName(roomName);
      updateUrlParams({
        room_id: roomId,
        room_name: roomId ? roomName : "",
      });
    },
    [updateUrlParams],
  );

  const clearRoomFilter = useCallback(() => {
    updateRoomFilter("", "");
  }, [updateRoomFilter]);

  const updateSelectedYear = useCallback(
    (value: string) => {
      setSelectedYear(value);
      updateUrlParams({ year: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateSelectedSchoolClass = useCallback(
    (value: string) => {
      setSelectedSchoolClass(value);
      updateUrlParams({ school_class: value });
    },
    [updateUrlParams],
  );

  const updateSelectedGroup = useCallback(
    (value: string) => {
      const normalizedGroupId = normalizeSearchStudentsGroupId(value);
      setSelectedGroup(normalizedGroupId);
      updateUrlParams({ group_id: normalizedGroupId });
    },
    [updateUrlParams],
  );

  const updateAttendanceFilter = useCallback(
    (value: StatusFilter) => {
      setAttendanceFilter(value);
      updateUrlParams({ status: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateBusFilter = useCallback(
    (value: BooleanFilter) => {
      setBusFilter(value);
      updateUrlParams({ bus: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updatePhotoConsentFilter = useCallback(
    (value: BooleanFilter) => {
      setPhotoConsentFilter(value);
      updateUrlParams({ photo_consent: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updatePickupStatusFilter = useCallback(
    (value: PickupStatusFilter) => {
      setPickupStatusFilter(value);
      updateUrlParams({ pickup_status: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updatePickupTimeFilter = useCallback(
    (value: string) => {
      setPickupTimeFilter(value);
      updateUrlParams({ pickup_time: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateArrivalTimeFilter = useCallback(
    (value: string) => {
      setArrivalTimeFilter(value);
      updateUrlParams({ arrival_time: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateDayStatusFilter = useCallback(
    (value: DayStatusFilter) => {
      setDayStatusFilter(value);
      updateUrlParams({ day_status: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateTrackingFilter = useCallback(
    (value: TrackingFilter) => {
      setTrackingFilter(value);
      updateUrlParams({ tracking: value === "all" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateSortMode = useCallback(
    (value: SortMode) => {
      setSortMode(value);
      updateUrlParams({ sort: value === "name" ? "" : value });
    },
    [updateUrlParams],
  );

  const updateGroupMode = useCallback(
    (value: GroupMode) => {
      setGroupMode(value);
      updateUrlParams({ view: value === "none" ? "" : value });
    },
    [updateUrlParams],
  );

  const clearAllFilters = useCallback(() => {
    setSearchTerm("");
    setSelectedGroup("");
    setSelectedSchoolClass("");
    setSelectedYear("all");
    setAttendanceFilter("all");
    setBusFilter("all");
    setPhotoConsentFilter("all");
    setPickupStatusFilter("all");
    setPickupTimeFilter("all");
    setArrivalTimeFilter("all");
    setDayStatusFilter("all");
    setTrackingFilter("all");
    setSortMode("name");
    setGroupMode("none");
    setSelectedRoomId("");
    setSelectedRoomName("");
    setExplicitDate(null);
    updateUrlParams({
      ...Object.fromEntries(FILTER_QUERY_PARAMS.map((key) => [key, ""])),
      [DATE_QUERY_PARAM]: "",
    });
    removeStoredFilters(storageKey);
  }, [storageKey, updateUrlParams]);

  // keepPreviousData holds the previous request's rows until the new request
  // resolves. Gate every date-sensitive derivation on the response's own
  // requestDate AND today/planning mode so prior statuses, counts, and filter
  // options are never presented under the newly selected date/mode (#1939).
  // Non-date filter changes keep both tags equal, so only date OR mode switches
  // gate. The mode tag is what catches the midnight rollover: selectedDate is
  // unchanged when a future date becomes today, but requestIsToday flips, so a
  // held planning response is still recognized as stale. An untagged response
  // (requestDate absent, e.g. under test mocks) is treated as current, so this
  // never gates a real fetch — only an in-flight date/mode transition.
  const isDateTransition =
    studentsData?.requestDate !== undefined &&
    (studentsData.requestDate !== selectedDate ||
      studentsData.requestIsToday !== isToday);
  const students = useMemo(
    () =>
      studentsData === undefined || isDateTransition
        ? EMPTY_STUDENT_ARRAY
        : studentsData.students,
    [studentsData, isDateTransition],
  );
  const photoConsentDataAvailable =
    photoConsentFeatureAvailable &&
    (students.length === 0 ||
      students.some(
        (student) =>
          student.has_full_access !== false &&
          student.photo_consent_given !== undefined,
      ));
  const canUsePhotoConsentFilter =
    studentPhotosSettingLoading || photoConsentDataAvailable;
  const effectivePhotoConsentFilter = canUsePhotoConsentFilter
    ? photoConsentFilter
    : "all";

  useEffect(() => {
    if (!canUsePhotoConsentFilter && photoConsentFilter !== "all") {
      updatePhotoConsentFilter("all");
    }
  }, [canUsePhotoConsentFilter, photoConsentFilter, updatePhotoConsentFilter]);

  // Tracking indicators for student cards
  const trackingStudentIds = useMemo(
    () => students.map((s) => s.id),
    [students],
  );
  const trackingStudentIdsKey = useMemo(
    () => trackingStudentIds.join(","),
    [trackingStudentIds],
  );
  // Tracking indicators (Mensa/HA) describe TODAY's activity participation.
  // Every consumer below is already gated on isToday — the filter is
  // neutralized (effectiveTrackingFilter), its option is dropped from the
  // filter set, and the card badges are not rendered — so on a planning date
  // the request is pure waste. Skip it entirely (#1939).
  const { data: trackingData } = useSWRAuth<TrackingIndicatorsResponse>(
    isToday && trackingStudentIds.length > 0
      ? `tracking-indicators-${trackingStudentIdsKey}`
      : null,
    async () => activeService.getTrackingIndicators(trackingStudentIds),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Reset tracking filter if labels vanish or the selected label index becomes stale.
  // Without this, the active-filter chip (guarded by trackingData.labels) can hide
  // while trackingFilter remains set, leaving an invisible filter applied.
  const trackingLabels = trackingData?.labels;
  useEffect(() => {
    const next = resolveTrackingFilterAfterLabelChange(
      trackingFilter,
      trackingData,
    );
    if (next !== trackingFilter) updateTrackingFilter(next);
  }, [trackingData, trackingFilter, updateTrackingFilter]);

  // Error type for proper heading display (Fix P3: substring matching on transformed string)
  type ErrorType = "permission" | "session" | "generic" | null;

  // Parse error messages for user-friendly display, returning both type and message
  const [errorType, errorMessage]: [ErrorType, string | null] = useMemo(() => {
    if (!studentsError) return [null, null];

    const rawMessage =
      studentsError instanceof Error
        ? studentsError.message
        : String(studentsError);

    if (rawMessage.includes("403")) {
      return [
        "permission",
        "Du hast keine Berechtigung, Kinderdaten anzuzeigen. Bitte wende dich an einen Administrator.",
      ];
    }
    if (rawMessage.includes("401")) {
      return [
        "session",
        "Deine Sitzung ist abgelaufen. Bitte melde dich erneut an.",
      ];
    }
    return ["generic", "Fehler beim Laden der Kinderdaten."];
  }, [studentsError]);

  // Fix P1: Detect when auth prevents fetching (user can't fetch but no error from SWR)
  const canFetch = status === "authenticated" && !!session?.user?.token;
  const isAuthError = !canFetch && !studentsError && status !== "loading";

  // Fix P2: Track initialization state to prevent empty state flash
  // Only wait for session - user context loads in parallel (for badge styling only)
  const isInitializing = status === "loading";
  const hasFetchedOnce =
    studentsData !== undefined || studentsError !== undefined;

  // SSE is handled globally by TenantAuthWrapper - no page-level setup needed.
  // When student_checkin/checkout events occur, global SSE invalidates "student*" caches,
  // which triggers SWR refetch for search-students-* keys automatically.

  // Prepare filter configurations for PageHeaderWithSearch
  const orderedRoomOptions = useMemo(
    () => orderedRooms(rooms, myGroupRooms, mySupervisedRooms),
    [rooms, myGroupRooms, mySupervisedRooms],
  );

  // Grouping for the quiet filter popover. Lives here, not in the shared
  // panel: which student filters belong together is this page's domain
  // knowledge. Any id not listed falls into a trailing "Weitere" section.
  const filterSections: FilterSection[] = useMemo(
    () => [
      {
        title: "Organisation",
        icon: DetailIcons.building,
        filterIds: ["year", "schoolClass", "group", "room"],
      },
      {
        title: "Anwesenheit",
        icon: DetailIcons.check,
        filterIds: ["planningDate", "dayStatus", "attendance", "tracking"],
      },
      {
        title: "Zeiten & Abholung",
        icon: DetailIcons.bus,
        filterIds: [
          "arrivalTime",
          "pickupTime",
          "pickupStatus",
          "bus",
          "photoConsent",
        ],
      },
      {
        title: "Darstellung",
        icon: DetailIcons.document,
        filterIds: ["sort", "groupMode"],
      },
    ],
    [],
  );

  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "year",
        label: "Stufe",
        type: "dropdown",
        value: selectedYear,
        onChange: (value) => updateSelectedYear(value as string),
        options: SCHOOL_YEAR_DROPDOWN_OPTIONS,
      },
      {
        id: "schoolClass",
        label: "Klasse",
        type: "dropdown",
        value: selectedSchoolClass,
        onChange: (value) => updateSelectedSchoolClass(value as string),
        options: [
          { value: "", label: "Alle Klassen" },
          ...schoolClassOptions.map((schoolClass) => ({
            value: schoolClass,
            label: schoolClass,
          })),
          ...(selectedSchoolClass &&
          !schoolClassOptions.some(
            (schoolClass) => schoolClass === selectedSchoolClass,
          )
            ? [{ value: selectedSchoolClass, label: selectedSchoolClass }]
            : []),
        ],
      },
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: selectedGroup,
        onChange: (value) => updateSelectedGroup(value as string),
        options: [
          { value: "", label: "Alle Gruppen" },
          ...groups.map((group) => ({ value: group.id, label: group.name })),
        ],
      },
      // The room filter reads current visits — today-only by nature (#1939).
      ...(isToday
        ? [
            {
              id: "room",
              label: "Raum",
              type: "dropdown" as const,
              value: selectedRoomId,
              onChange: (value: string | string[]) => {
                const v = Array.isArray(value) ? value[0] : value;
                if (!v) {
                  clearRoomFilter();
                  return;
                }
                const room = rooms.find((r) => r.id === v);
                updateRoomFilter(
                  v,
                  room?.name ?? (v === selectedRoomId ? selectedRoomName : ""),
                );
              },
              options: [
                { value: "", label: "Alle Räume" },
                ...orderedRoomOptions.map((room) => ({
                  value: room.id,
                  label: room.name,
                })),
                ...(selectedRoomId &&
                !orderedRoomOptions.some((room) => room.id === selectedRoomId)
                  ? [
                      {
                        value: selectedRoomId,
                        label: selectedRoomName || `Raum #${selectedRoomId}`,
                      },
                    ]
                  : []),
              ],
            },
          ]
        : []),
      {
        id: "pickupTime",
        label: "Gehzeit",
        type: "dropdown",
        value: pickupTimeFilter,
        onChange: (value) => updatePickupTimeFilter(value as string),
        options: [
          { value: "all", label: "Alle Gehzeiten" },
          ...Array.from(
            new Set(
              students
                .map((s) => s.pickup_time)
                .filter((t): t is string => !!t),
            ),
          )
            .sort((a, b) => a.localeCompare(b))
            .map((time) => ({ value: time, label: `${time} Uhr` })),
          { value: "none", label: "Keine Gehzeit" },
        ],
      },
      {
        id: "arrivalTime",
        label: "Ankunftszeit",
        type: "dropdown",
        value: arrivalTimeFilter,
        onChange: (value) => updateArrivalTimeFilter(value as string),
        options: [
          { value: "all", label: "Alle Ankunftszeiten" },
          ...Array.from(
            new Set(
              students
                .map((s) => s.arrival_time)
                .filter((t): t is string => !!t),
            ),
          )
            .sort((a, b) => a.localeCompare(b))
            .map((time) => ({ value: time, label: `${time} Uhr` })),
          { value: "none", label: "Keine Ankunftszeit" },
        ],
      },
      // Planning-day chooser (#1939). A custom filter so it lives *inside* the
      // "Anwesenheit" section, directly above the Kommt/Kommt-nicht filter it
      // scopes — the date is the reference day for that filter, not a separate
      // mode. Spans both grid columns so the inline calendar has room. Empty
      // value/onChange/options are the required-but-unused stubs for a custom
      // filter (see FilterConfig.render).
      {
        id: "planningDate",
        label: "Tag",
        type: "custom" as const,
        value: "",
        onChange: () => undefined,
        options: [],
        className: "md:col-span-2",
        render: (
          <div className="space-y-2">
            <div className="flex flex-wrap gap-1.5">
              <Button
                type="button"
                size="compact"
                variant={isToday ? "primary" : "outline"}
                onClick={() => updateSelectedDate(todayIso)}
              >
                Heute
              </Button>
              {/* On a Sunday the horizon closes with today (see
                  maxPlanningIso), so tomorrow is not selectable — hide the
                  shortcut instead of offering one that clamps back to today. */}
              {tomorrowIso <= maxDateIso && (
                <Button
                  type="button"
                  size="compact"
                  variant={selectedDate === tomorrowIso ? "primary" : "outline"}
                  onClick={() => updateSelectedDate(tomorrowIso)}
                >
                  Morgen
                </Button>
              )}
            </div>
            <DatePicker
              value={parseISODate(selectedDate)}
              minDate={parseISODate(todayIso)}
              maxDate={parseISODate(maxDateIso)}
              calendarLayout="popover"
              onChange={(date) =>
                updateSelectedDate(date ? toISODate(date) : todayIso)
              }
            />
            {!isToday && (
              <p className="text-xs text-gray-500">
                Geplante Anwesenheit für {formatDate(selectedDate, true)},
                basierend auf Anwesenheits- und Betreuungsplänen sowie
                gemeldeten Abwesenheiten. Aktuelle Aufenthaltsorte und
                Live-Filter bleiben ausgeblendet.
              </p>
            )}
          </div>
        ),
      },
      {
        id: "dayStatus",
        label: isToday ? "Tagesplanung" : "Kommt / Kommt nicht",
        type: "dropdown",
        value: dayStatusFilter,
        onChange: (value) => updateDayStatusFilter(value as DayStatusFilter),
        options: dayStatusFilterOptions,
      },
      // The location-derived buckets read current whereabouts and drop out for
      // another day; the absence buckets follow the selected date (#1939).
      {
        id: "attendance",
        label: "Status",
        type: "dropdown" as const,
        value: effectiveAttendanceFilter,
        onChange: (value: string | string[]) =>
          updateAttendanceFilter(value as StatusFilter),
        options: [
          { value: "all", label: "Alle Status" },
          ...(isToday ? LIVE_STATUS_FILTER_OPTIONS : []),
          ...PLANNED_ABSENCE_STATUS_FILTER_OPTIONS,
        ],
      },
      {
        id: "bus",
        label: "Buskind",
        type: "dropdown",
        value: busFilter,
        onChange: (value) => updateBusFilter(value as BooleanFilter),
        options: BUS_FILTER_OPTIONS,
      },
      ...(canUsePhotoConsentFilter
        ? [
            {
              id: "photoConsent",
              label: "Fotoerlaubnis",
              type: "dropdown" as const,
              value: photoConsentFilter,
              onChange: (value: string | string[]) =>
                updatePhotoConsentFilter(value as BooleanFilter),
              options: PHOTO_CONSENT_FILTER_OPTIONS,
            },
          ]
        : []),
      {
        id: "pickupStatus",
        label: "Abholregelung",
        type: "dropdown",
        value: pickupStatusFilter,
        onChange: (value) =>
          updatePickupStatusFilter(value as PickupStatusFilter),
        options: PICKUP_STATUS_FILTER_OPTIONS,
      },
      {
        id: "sort",
        label: "Sortierung",
        type: "dropdown",
        value: sortMode,
        onChange: (value) => updateSortMode(value as SortMode),
        options: SORT_OPTIONS,
      },
      {
        id: "groupMode",
        label: "Ansicht",
        type: "dropdown",
        value: effectiveGroupMode,
        onChange: (value) => updateGroupMode(value as GroupMode),
        // Status/room grouping reads current whereabouts — today-only (#1939).
        options: isToday
          ? GROUP_OPTIONS
          : GROUP_OPTIONS.filter(
              (option) => option.value !== "status" && option.value !== "room",
            ),
      },
      ...(isToday && trackingLabels && trackingLabels.length > 0
        ? [
            {
              id: "tracking",
              label: "Aktivitäten heute",
              type: "dropdown" as const,
              value: trackingFilter,
              onChange: (value: string | string[]) =>
                updateTrackingFilter(value as TrackingFilter),
              options: [
                { value: "all", label: "Alle Aktivitäten heute" },
                ...trackingLabels.map((lbl, idx) => ({
                  value: `missing:${idx}`,
                  label: `Noch nicht in ${lbl}`,
                })),
                { value: "none_visited", label: "Noch nichts erledigt" },
              ],
            },
          ]
        : []),
    ],
    [
      selectedYear,
      selectedSchoolClass,
      selectedGroup,
      pickupTimeFilter,
      arrivalTimeFilter,
      busFilter,
      photoConsentFilter,
      pickupStatusFilter,
      canUsePhotoConsentFilter,
      dayStatusFilter,
      trackingFilter,
      trackingLabels,
      groups,
      schoolClassOptions,
      rooms,
      students,
      selectedRoomId,
      selectedRoomName,
      orderedRoomOptions,
      clearRoomFilter,
      updateRoomFilter,
      sortMode,
      updateSelectedYear,
      updateSelectedSchoolClass,
      updateSelectedGroup,
      updateAttendanceFilter,
      updateBusFilter,
      updatePhotoConsentFilter,
      updatePickupStatusFilter,
      updatePickupTimeFilter,
      updateArrivalTimeFilter,
      updateDayStatusFilter,
      updateTrackingFilter,
      updateSortMode,
      updateGroupMode,
      isToday,
      dayStatusFilterOptions,
      effectiveAttendanceFilter,
      effectiveGroupMode,
      selectedDate,
      todayIso,
      tomorrowIso,
      maxDateIso,
      updateSelectedDate,
    ],
  );

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    // A non-today planning day is an active filter state (#1939): it re-scopes
    // the Kommt/Kommt-nicht filter and the whole list to another day. Surface
    // it so the filter-count badge accounts for it; removing it returns to
    // today.
    if (!isToday) {
      filters.push({
        id: "planningDate",
        label: `Tag: ${formatDate(selectedDate, true)}`,
        onRemove: () => updateSelectedDate(todayIso),
      });
    }

    if (selectedYear !== "all") {
      filters.push({
        id: "year",
        label: `Jahr ${selectedYear}`,
        onRemove: () => updateSelectedYear("all"),
      });
    }

    if (selectedSchoolClass) {
      filters.push({
        id: "schoolClass",
        label: `Klasse ${selectedSchoolClass}`,
        onRemove: () => updateSelectedSchoolClass(""),
      });
    }

    if (selectedGroup) {
      const groupName =
        groups.find((g) => g.id === selectedGroup)?.name ?? "Gruppe";
      filters.push({
        id: "group",
        label: groupName,
        onRemove: () => updateSelectedGroup(""),
      });
    }

    if (effectiveRoomId) {
      // Fall back to "Raum #{id}" when no room_name was passed in the URL
      // (e.g. an old bookmark), better than rendering an empty chip.
      const label = selectedRoomName
        ? `Raum: ${selectedRoomName}`
        : `Raum #${effectiveRoomId}`;
      filters.push({
        id: "room",
        label,
        onRemove: clearRoomFilter,
      });
    }

    if (effectiveAttendanceFilter !== "all") {
      const statusLabels: Record<Exclude<StatusFilter, "all">, string> = {
        anwesend: "Anwesend",
        abwesend: "Abwesend",
        unterwegs: "Unterwegs",
        schulhof: "Schulhof",
        krank: "Krank",
        klassenfahrt: "Klassenfahrt",
        entschuldigt: "Entschuldigt",
      };
      filters.push({
        id: "attendance",
        label:
          statusLabels[effectiveAttendanceFilter] ?? effectiveAttendanceFilter,
        onRemove: () => updateAttendanceFilter("all"),
      });
    }

    if (busFilter !== "all") {
      filters.push({
        id: "bus",
        label: busFilter === "yes" ? "Buskind" : "Kein Buskind",
        onRemove: () => updateBusFilter("all"),
      });
    }

    if (effectivePhotoConsentFilter !== "all") {
      filters.push({
        id: "photoConsent",
        label:
          effectivePhotoConsentFilter === "yes"
            ? "Fotoerlaubnis liegt vor"
            : "Keine Fotoerlaubnis",
        onRemove: () => updatePhotoConsentFilter("all"),
      });
    }

    if (pickupStatusFilter !== "all") {
      filters.push({
        id: "pickupStatus",
        label:
          PICKUP_STATUS_FILTER_OPTIONS.find(
            (option) => option.value === pickupStatusFilter,
          )?.label ?? "Abholregelung",
        onRemove: () => updatePickupStatusFilter("all"),
      });
    }

    if (dayStatusFilter !== "all") {
      filters.push({
        id: "dayStatus",
        label:
          dayStatusFilterOptions.find(
            (option) => option.value === dayStatusFilter,
          )?.label ?? dayStatusFilter,
        onRemove: () => updateDayStatusFilter("all"),
      });
    }

    if (pickupTimeFilter !== "all") {
      filters.push({
        id: "pickupTime",
        label:
          pickupTimeFilter === "none"
            ? "Keine Gehzeit"
            : `Gehzeit ${pickupTimeFilter} Uhr`,
        onRemove: () => updatePickupTimeFilter("all"),
      });
    }

    if (arrivalTimeFilter !== "all") {
      filters.push({
        id: "arrivalTime",
        label:
          arrivalTimeFilter === "none"
            ? "Keine Ankunftszeit"
            : `Ankunftszeit ${arrivalTimeFilter} Uhr`,
        onRemove: () => updateArrivalTimeFilter("all"),
      });
    }

    if (trackingLabels && isToday) {
      const chipLabel = trackingFilterChipLabel(trackingFilter, trackingLabels);
      if (chipLabel !== null) {
        filters.push({
          id: "tracking",
          label: chipLabel,
          onRemove: () => updateTrackingFilter("all"),
        });
      }
    }

    if (sortMode !== "name") {
      filters.push({
        id: "sort",
        label:
          SORT_OPTIONS.find((option) => option.value === sortMode)?.label ??
          "Sortierung",
        onRemove: () => updateSortMode("name"),
      });
    }

    if (effectiveGroupMode !== "none") {
      filters.push({
        id: "groupMode",
        label: `Ansicht: ${
          GROUP_OPTIONS.find((option) => option.value === effectiveGroupMode)
            ?.label ?? "Ansicht"
        }`,
        onRemove: () => updateGroupMode("none"),
      });
    }

    return filters;
  }, [
    searchTerm,
    selectedYear,
    selectedSchoolClass,
    selectedGroup,
    busFilter,
    effectivePhotoConsentFilter,
    pickupStatusFilter,
    dayStatusFilter,
    pickupTimeFilter,
    arrivalTimeFilter,
    trackingFilter,
    trackingLabels,
    groups,
    selectedRoomName,
    sortMode,
    clearRoomFilter,
    updateSelectedYear,
    updateSelectedSchoolClass,
    updateSelectedGroup,
    updateAttendanceFilter,
    updateBusFilter,
    updatePhotoConsentFilter,
    updatePickupStatusFilter,
    updateDayStatusFilter,
    updatePickupTimeFilter,
    updateArrivalTimeFilter,
    updateTrackingFilter,
    updateSortMode,
    updateGroupMode,
    isToday,
    dayStatusFilterOptions,
    effectiveAttendanceFilter,
    effectiveRoomId,
    effectiveGroupMode,
    selectedDate,
    todayIso,
    updateSelectedDate,
  ]);

  const exportFilters = useMemo(
    () => ({
      search: searchTerm,
      group_id: selectedGroup,
      year: selectedYear,
      school_class: selectedSchoolClass,
      // The realtime snapshot/room filters are neutral for non-today dates —
      // the export must mirror what the page shows (#1939).
      status: effectiveAttendanceFilter,
      bus: busFilter,
      photo_consent: effectivePhotoConsentFilter,
      pickup_status: pickupStatusFilter,
      day_status: dayStatusFilter,
      ...(isToday ? {} : { date: selectedDate }),
      pickup_time: pickupTimeFilter,
      arrival_time: arrivalTimeFilter,
      room_id: effectiveRoomId,
      sort: sortMode,
    }),
    [
      searchTerm,
      selectedGroup,
      selectedYear,
      selectedSchoolClass,
      effectiveAttendanceFilter,
      busFilter,
      effectivePhotoConsentFilter,
      pickupStatusFilter,
      dayStatusFilter,
      isToday,
      selectedDate,
      pickupTimeFilter,
      arrivalTimeFilter,
      effectiveRoomId,
      sortMode,
    ],
  );

  // Apply additional client-side filtering for attendance statuses and year
  const filteredStudents: Student[] = students.filter((student) => {
    // Apply attendance filter (neutralized for non-today planning dates)
    if (effectiveAttendanceFilter !== "all") {
      const isOnSite =
        isPresentLocation(student.current_location) ||
        isTransitLocation(student.current_location) ||
        isSchoolyardLocation(student.current_location);

      if (effectiveAttendanceFilter === "anwesend" && !isOnSite) {
        return false;
      }

      if (
        effectiveAttendanceFilter !== "anwesend" &&
        statusLabelForStudent(student) !==
          STATUS_FILTER_LABELS[effectiveAttendanceFilter]
      ) {
        return false;
      }
    }

    if (
      dayStatusFilter !== "all" &&
      student.day_planning_status !== dayStatusFilter
    ) {
      return false;
    }

    // Apply year filter - extract year from school_class (e.g., "Klasse 3a" → year 3)
    if (selectedYear !== "all") {
      const studentYear = getSchoolYear(student.school_class);
      if (studentYear !== selectedYear) {
        return false;
      }
    }

    // bus / photo_consent / pickup_status (#1492) are filtered server-side now
    // (see api/students applyAdministrativeFilters), so the page no longer
    // re-filters them here. The fetched page is already the filtered set.

    // Apply pickup time filter. For redacted students, missing pickup_time
    // due to has_full_access=false is not the same as "no schedule")
    if (pickupTimeFilter !== "all") {
      if (student.has_full_access === false) return false;
      if (pickupTimeFilter === "none") {
        if (student.pickup_time || student.pickup_is_exception) return false;
      } else if (student.pickup_time !== pickupTimeFilter) {
        return false;
      }
    }

    if (arrivalTimeFilter !== "all") {
      if (student.has_full_access === false) return false;
      if (arrivalTimeFilter === "none") {
        if (student.arrival_time || student.arrival_is_exception) return false;
      } else if (student.arrival_time !== arrivalTimeFilter) {
        return false;
      }
    }

    if (
      !matchesTrackingFilter(student, effectiveTrackingFilter, trackingData)
    ) {
      return false;
    }

    return true;
  });

  // Apply sort mode (name = stable A-Z; arrival/pickup = daily operational order)
  const sortedStudents = useMemo(() => {
    if (sortMode === "name") return [...filteredStudents].sort(compareByName);

    return [...filteredStudents].sort((a, b) => {
      if (sortMode === "pickup") {
        return compareByPickupTime(a, b, planningNow);
      }

      // Whether a child is currently at home only orders today's list; for a
      // non-today planning date the live location is irrelevant.
      if (isToday) {
        const aHome = isHomeLocation(a.current_location);
        const bHome = isHomeLocation(b.current_location);
        if (!aHome && bHome) return 1;
        if (aHome && !bHome) return -1;
      }

      const statusA = getStudentTimeStatus({
        plannedTime: a.arrival_time,
        actualTime: a.actual_arrival_time,
        now: planningNow,
        sick: a.sick,
        classTrip: a.class_trip,
        excused: a.excused,
      });
      const statusB = getStudentTimeStatus({
        plannedTime: b.arrival_time,
        actualTime: b.actual_arrival_time,
        now: planningNow,
        sick: b.sick,
        classTrip: b.class_trip,
        excused: b.excused,
      });
      const rankA = getTimeStatusSortRank(statusA);
      const rankB = getTimeStatusSortRank(statusB);
      if (rankA !== rankB) return rankA - rankB;

      if (a.arrival_time && b.arrival_time) {
        const timeCmp = a.arrival_time.localeCompare(b.arrival_time);
        if (timeCmp !== 0) return timeCmp;
      }
      return compareByName(a, b);
    });
  }, [filteredStudents, sortMode, planningNow, isToday]);

  const groupedStudents = useMemo(
    () => groupStudents(sortedStudents, effectiveGroupMode),
    [sortedStudents, effectiveGroupMode],
  );

  // Fix P2: Show loading during initialization (prevents empty state flash)
  // Note: With required: true, unauthenticated users are auto-redirected to login
  if (isInitializing || isAuthError) {
    return <StudentSearchPageSkeleton />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Page header scrolls with the rest of the page (no sticky).
          Active filters surface as a count badge on the filter pill. The
          check-in/out trigger lives in a floating FAB rendered at the
          bottom of this component on mobile/tablet, or inline in the
          header on desktop via primaryAction. */}
      <div className="-mx-1 px-1 pb-2 sm:mx-0 sm:px-0">
        <PageHeaderWithSearch
          title="Kindersuche"
          badge={{
            icon: (
              <svg
                className="h-5 w-5 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
            ),
            count: filteredStudents.length,
          }}
          primaryAction={
            isBinaryMode && isToday ? (
              <SchoolCheckinFab
                variant="inline"
                isActive={schoolCheckin.isActive}
                onToggle={schoolCheckin.toggleActive}
                successCount={schoolCheckin.successCount}
                pendingCount={schoolCheckin.pendingIds.size}
              />
            ) : undefined
          }
          // 6 filters overflow the inline desktop row at iPad-class
          // viewports. Switch to the mobile sheet pattern up to xl
          // (1280px). Matches Stripe / Airbnb / Slack pattern for
          // filter-heavy pages.
          desktopFiltersFrom="xl"
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Name suchen...",
          }}
          filters={filterConfigs}
          activeFilters={activeFilters}
          activeFilterDisplay="count"
          filterVariant="quiet"
          filterSections={filterSections}
          onClearAllFilters={clearAllFilters}
          overflowMenu={[
            {
              label: "Exportieren",
              icon: <Download className="h-4 w-4" aria-hidden />,
              onClick: () => setIsExportOpen(true),
              badge: filteredStudents.length,
            },
          ]}
        />
      </div>

      {/* Planning-date context banner (#1939). The day chooser itself lives in
          the filter panel, in the "Anwesenheit" section right above the
          Kommt/Kommt-nicht filter it scopes. This banner only appears for a
          non-today date so nobody mistakes a plan view for live presence. */}
      {!isToday && (
        <div className="mb-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-600">
          Geplante Anwesenheit für {formatDate(selectedDate, true)}; aktuelle
          Aufenthaltsorte bleiben ausgeblendet.{" "}
          <button
            type="button"
            onClick={() => updateSelectedDate(todayIso)}
            className="font-medium text-gray-900 underline underline-offset-2 hover:text-gray-700"
          >
            Zurück zu heute
          </button>
        </div>
      )}

      {/* Mobile Error Display, outside the sticky stack so it doesn't
          push everything down on small screens. */}
      {errorMessage && (
        <div className="mb-4 md:hidden">
          <Alert type="error" message={errorMessage} />
        </div>
      )}

      {/* Mobile (<md) check-in mode trigger, inline pill / sticky bar.
          Check-in toggles TODAY's attendance, so it hides on other dates. */}
      {isBinaryMode && isToday && (
        <div className="mb-3 md:hidden">
          <SchoolCheckinModeMobile
            isActive={schoolCheckin.isActive}
            onToggle={schoolCheckin.toggleActive}
            successCount={schoolCheckin.successCount}
            pendingCount={schoolCheckin.pendingIds.size}
          />
        </div>
      )}

      {/* Student Grid. Bottom padding reserves room for the mobile sticky
          bar / tablet floating FAB; desktop has neither. */}
      <div className={isBinaryMode ? "pb-24 lg:pb-0" : undefined}>
        {(() => {
          // Fix P2: Show loading while first fetch is in progress (not yet hasFetchedOnce)
          // or while a date switch is in flight — keepPreviousData still holds
          // the previous day's rows, so show the skeleton instead of presenting
          // them under the newly selected date (#1939). Handle a fetch error
          // FIRST, though: when the new date's request fails, keepPreviousData
          // keeps the stale response so isDateTransition stays true — without
          // this guard the page would show the skeleton indefinitely instead of
          // the error and its recovery guidance (#1939).
          if (
            !errorMessage &&
            ((isSearching && !hasFetchedOnce) || isDateTransition)
          ) {
            return <StudentCardGridSkeleton />;
          }
          if (errorMessage) {
            return (
              <div className="py-12 text-center">
                <div className="flex flex-col items-center gap-4">
                  <svg
                    className="h-12 w-12 text-red-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <div>
                    {/* Fix P3: Use errorType instead of substring matching */}
                    <h3 className="text-lg font-medium text-gray-900">
                      {errorType === "permission"
                        ? "Keine Berechtigung"
                        : "Fehler"}
                    </h3>
                    <p className="text-gray-600">{errorMessage}</p>
                  </div>
                </div>
              </div>
            );
          }
          // Fix P2: Only show empty state if we've fetched at least once
          if (filteredStudents.length === 0 && hasFetchedOnce) {
            return (
              <div className="py-12 text-center">
                <div className="flex flex-col items-center gap-4">
                  <svg
                    className="h-12 w-12 text-gray-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                    />
                  </svg>
                  <div>
                    <h3 className="text-lg font-medium text-gray-900">
                      Keine Kinder gefunden
                    </h3>
                    <p className="text-gray-600">
                      Versuche deine Suchkriterien anzupassen.
                    </p>
                  </div>
                </div>
              </div>
            );
          }
          // Preserve the URL-rehydrating filters in the back-link so stepping
          // from the search page → child → back returns to the same operational
          // view. Free-text search intentionally remains transient.
          const buildFromParam = (() => {
            const qs = new URLSearchParams();
            if (effectiveRoomId) {
              qs.set("room_id", effectiveRoomId);
              if (selectedRoomName) qs.set("room_name", selectedRoomName);
            }
            if (selectedGroup) qs.set("group_id", selectedGroup);
            if (selectedYear !== "all") qs.set("year", selectedYear);
            if (effectiveAttendanceFilter !== "all")
              qs.set("status", effectiveAttendanceFilter);
            if (busFilter !== "all") qs.set("bus", busFilter);
            if (effectivePhotoConsentFilter !== "all")
              qs.set("photo_consent", effectivePhotoConsentFilter);
            if (pickupStatusFilter !== "all")
              qs.set("pickup_status", pickupStatusFilter);
            if (dayStatusFilter !== "all")
              qs.set("day_status", dayStatusFilter);
            if (!isToday) qs.set("date", selectedDate);
            if (pickupTimeFilter !== "all")
              qs.set("pickup_time", pickupTimeFilter);
            if (arrivalTimeFilter !== "all")
              qs.set("arrival_time", arrivalTimeFilter);
            if (effectiveTrackingFilter !== "all")
              qs.set("tracking", effectiveTrackingFilter);
            if (sortMode !== "name") qs.set("sort", sortMode);
            if (effectiveGroupMode !== "none")
              qs.set("view", effectiveGroupMode);
            if (qs.size === 0) return "/students/search";
            return encodeURIComponent(`/students/search?${qs.toString()}`);
          })();
          const renderStudentCard = (student: Student) => {
            const checkinState = deriveCheckinState(student.current_location);
            const studentIdStr = student.id.toString();
            return (
              <StudentCard
                key={student.id}
                studentId={student.id}
                firstName={student.first_name}
                lastName={student.second_name}
                photoUrl={student.photo_url ?? null}
                onClick={() =>
                  router.push(`/students/${student.id}?from=${buildFromParam}`)
                }
                checkinMode={isBinaryMode && isToday && schoolCheckin.isActive}
                checkinState={checkinState}
                isCheckinPending={schoolCheckin.pendingIds.has(studentIdStr)}
                onCheckinClick={() =>
                  void schoolCheckin.toggle(studentIdStr, checkinState)
                }
                locationBadge={
                  isToday ? (
                    <StudentPresenceBadge
                      student={(() => {
                        const badgePlanning =
                          getStudentPresenceBadgePlanning(student);
                        return {
                          ...student,
                          not_arrival_today: badgePlanning.notArrivalToday,
                          not_arrival_reason: badgePlanning.notArrivalReason,
                        };
                      })()}
                      displayMode="contextAware"
                      userGroups={myGroups}
                      groupRooms={myGroupRooms}
                      supervisedRooms={mySupervisedRooms}
                      variant="modern"
                      size="md"
                    />
                  ) : (
                    // Non-today dates show the planned expectation, never the
                    // live location (#1939). When the caller lacks full access
                    // the backend skips day-planning enrichment and omits
                    // day_planning_status; render an unknown state rather than
                    // asserting "Kommt nicht" for a result that was never
                    // calculated or disclosed.
                    <DataTableStatusBadge
                      active={student.day_planning_status === "comes_today"}
                      unknown={student.day_planning_status === undefined}
                      activeLabel="Kommt"
                      inactiveLabel="Kommt nicht"
                      unknownLabel="Keine Angabe"
                    />
                  )
                }
                extraContent={
                  <>
                    <StudentInfoRow icon={<SchoolClassIcon />}>
                      {student.school_class}
                    </StudentInfoRow>
                    {student.group_name && (
                      <StudentInfoRow icon={<GroupIcon />}>
                        Gruppe: {student.group_name}
                      </StudentInfoRow>
                    )}
                    {student.has_full_access !== false &&
                      student.pending_excused_note !== undefined && (
                        <StudentPendingExcusedRow
                          note={student.pending_excused_note}
                        />
                      )}
                    {student.has_full_access !== false &&
                      (() => {
                        const absence = getStudentAbsence({
                          sick: student.sick,
                          classTrip: student.class_trip,
                          excused: student.excused,
                        });
                        const absenceWording = isToday
                          ? undefined
                          : "Kommt nicht";
                        if (absence && !student.actual_pickup_time) {
                          return (
                            <StudentAbsenceRow
                              label={absence.label}
                              wording={absenceWording}
                            />
                          );
                        }
                        const dayPlanningNotComingLabel =
                          getDayPlanningNotComingLabel(student, {
                            ignoreCurrentAttendance: !isToday,
                          });
                        if (
                          dayPlanningNotComingLabel &&
                          !student.actual_pickup_time
                        ) {
                          return (
                            <StudentAbsenceRow
                              label={dayPlanningNotComingLabel}
                              wording={absenceWording}
                            />
                          );
                        }
                        return (
                          <>
                            <ArrivalTimeRow
                              arrivalTime={student.arrival_time}
                              actualTime={student.actual_arrival_time}
                              isException={
                                student.arrival_is_exception ?? false
                              }
                              isAbsent={
                                (student.arrival_is_exception ?? false) &&
                                !student.arrival_time
                              }
                              notes={student.arrival_notes}
                              now={planningNow}
                              absentWording={absenceWording}
                            />
                            <PickupTimeRow
                              pickupTime={student.pickup_time ?? undefined}
                              actualTime={student.actual_pickup_time}
                              isException={student.pickup_is_exception ?? false}
                              notes={student.pickup_notes}
                              now={planningNow}
                            />
                          </>
                        );
                      })()}
                  </>
                }
                trackingIndicators={
                  isToday &&
                  trackingData?.labels?.length &&
                  student.has_full_access !== false ? (
                    <TrackingIndicators
                      labels={trackingData.labels}
                      results={trackingData.results[student.id] ?? []}
                    />
                  ) : undefined
                }
              />
            );
          };

          if (effectiveGroupMode !== "none") {
            return (
              <div className="space-y-6">
                {groupedStudents.map((group) => (
                  <section key={group.key} data-testid="student-group">
                    <div className="mb-3 flex items-center gap-3">
                      <h2 className="text-sm font-semibold text-gray-900">
                        {group.label}
                      </h2>
                      <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                        {group.items.length}
                      </span>
                      <div className="h-px min-w-8 flex-1 bg-gray-200" />
                    </div>
                    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
                      {group.items.map(renderStudentCard)}
                    </div>
                  </section>
                ))}
              </div>
            );
          }

          return (
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
              {sortedStudents.map(renderStudentCard)}
            </div>
          );
        })()}
      </div>

      {/* Tablet (md..xl) check-in mode trigger, floating FAB. Tablet
          range is bumped to `xl` here to stay aligned with the header's
          desktopFiltersFrom="xl". Both the filter sheet and the FAB
          live under the same boundary so iPad Air gets the consistent
          tablet UX. */}
      {isBinaryMode && isToday && (
        <div className="hidden md:block xl:hidden">
          <SchoolCheckinFab
            variant="floating"
            floatingUntil="xl"
            isActive={schoolCheckin.isActive}
            onToggle={schoolCheckin.toggleActive}
            successCount={schoolCheckin.successCount}
            pendingCount={schoolCheckin.pendingIds.size}
          />
        </div>
      )}

      {isExportOpen && (
        <StudentExportModal
          isOpen={isExportOpen}
          filters={exportFilters}
          resultCount={filteredStudents.length}
          onClose={() => setIsExportOpen(false)}
        />
      )}
    </div>
  );
}

// Main component with Suspense wrapper
export default function StudentSearchPage() {
  return (
    <Suspense fallback={<StudentSearchPageSkeleton />}>
      <SearchPageContent />
    </Suspense>
  );
}
