"use client";

// components/students/student-table.tsx
// The table view of the pages that list children (#3834): Alle Kinder,
// Meine Gruppen and Aktuelle Aufsicht. One row per child, the same facts the
// Kinderkarte shows, one column each. Each page builds its columns from the
// helpers below, so a fact reads the same in every table.

import type { ComponentProps, ReactNode } from "react";

import { Avatar } from "~/components/ui/avatar";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import Link from "~/components/ui/navigation-link";
import {
  ArrivalTimeRow,
  PickupTimeRow,
  StudentAbsenceRow,
  StudentPendingExcusedRow,
} from "~/components/students/student-card";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import type { Student } from "~/lib/api";

/** A table column plus what the "Spalten" menu needs to know about it. */
export interface StudentTableColumn<T> extends DataTableColumn<T> {
  /** False for the name: without it a row names nobody. Default true. */
  readonly hideable?: boolean;
  /** Shown before the user ever chose. Default true. */
  readonly defaultVisible?: boolean;
}

type ArrivalRowProps = Omit<ComponentProps<typeof ArrivalTimeRow>, "variant">;
type PickupRowProps = Omit<ComponentProps<typeof PickupTimeRow>, "variant">;

/**
 * The day facts of one child, resolved by the page from wherever it keeps
 * arrival and pickup times. An absence replaces both times, as on the card.
 */
export interface StudentTableDay {
  readonly absence?: {
    readonly label: string;
    readonly wording?: string;
    readonly note?: string;
  };
  readonly arrival: ArrivalRowProps;
  readonly pickup: PickupRowProps;
}

export function studentFullName(student: Student): string {
  return (
    `${student.first_name ?? ""} ${student.second_name ?? ""}`.trim() ||
    student.name
  );
}

function NameCell({
  student,
  href,
  photosEnabled,
}: Readonly<{ student: Student; href: string; photosEnabled: boolean }>) {
  const name = studentFullName(student);
  return (
    <span className="flex min-w-0 items-center gap-3">
      {photosEnabled ? (
        <Avatar
          imageUrl={student.photo_url ?? null}
          name={name || "?"}
          size="sm"
          decorative
        />
      ) : null}
      {/* The link is the keyboard path into the child; a mouse click on the
          rest of the row opens it too. */}
      <Link
        href={href}
        className="min-w-0 font-medium break-words text-gray-900 hover:underline focus-visible:underline focus-visible:outline-none"
        onClick={(event) => event.stopPropagation()}
      >
        {name}
      </Link>
    </span>
  );
}

const EMPTY = <span className="text-gray-400">–</span>;

export function nameColumn<T extends Student>(
  hrefFor: (student: T) => string,
  photosEnabled: boolean,
): StudentTableColumn<T> {
  return {
    key: "name",
    header: "Name",
    className: "whitespace-nowrap",
    hideable: false,
    stacked: "title",
    render: (student) => (
      <NameCell
        student={student}
        href={hrefFor(student)}
        photosEnabled={photosEnabled}
      />
    ),
    sortValue: (student) =>
      `${student.second_name ?? ""} ${student.first_name ?? ""}`.toLowerCase(),
  };
}

export function classColumn<T extends Student>(
  options: { defaultVisible?: boolean } = {},
): StudentTableColumn<T> {
  return {
    key: "class",
    header: "Klasse",
    className: "whitespace-nowrap",
    stackedLabel: false,
    defaultVisible: options.defaultVisible,
    render: (student) => student.school_class || EMPTY,
    sortValue: (student) => student.school_class ?? "",
  };
}

export function groupColumn<T extends Student>(): StudentTableColumn<T> {
  return {
    key: "group",
    header: "Gruppe",
    className: "whitespace-nowrap",
    stackedLabel: false,
    render: (student) => student.group_name || EMPTY,
    sortValue: (student) => student.group_name ?? "",
  };
}

/**
 * Where the child is now (or, on a planning date, whether it comes). The page
 * renders its own badge because each page reads presence differently.
 */
export function statusColumn<T extends Student>(
  header: string,
  renderBadge: (student: T) => ReactNode,
): StudentTableColumn<T> {
  return {
    key: "status",
    header,
    className: "whitespace-nowrap",
    stacked: "meta",
    // One line per child on a phone: the badges side by side instead of
    // stacked. A second badge (Krank, Ungeplant anwesend) stays visible.
    stackedClassName:
      "[&>span]:flex-row [&>span]:flex-wrap [&>span]:items-center [&>span]:justify-end [&_.items-end]:flex-row [&_.items-end]:flex-wrap [&_.items-end]:justify-end [&_.items-end]:gap-1 [&_.mt-1]:mt-0 [&_span]:whitespace-nowrap",
    render: (student) => (
      // The badge stacks right-aligned for the card's top-right corner; in a
      // left-aligned column its stack starts at the left edge instead.
      <span className="flex flex-col items-start gap-1 [&_.items-end]:items-start">
        {renderBadge(student)}
        {student.has_full_access !== false &&
        student.pending_excused_note !== undefined ? (
          <StudentPendingExcusedRow note={student.pending_excused_note} />
        ) : null}
      </span>
    ),
  };
}

/** Day facts a caller without full access may not see. */
function canSeeDay(student: Student): boolean {
  return student.has_full_access !== false;
}

export function arrivalColumn<T extends Student>(
  getDay: (student: T) => StudentTableDay,
): StudentTableColumn<T> {
  return {
    key: "arrival",
    header: "Ankunft",
    render: (student) => {
      if (!canSeeDay(student)) return EMPTY;
      const day = getDay(student);
      if (day.absence) {
        return (
          <StudentAbsenceRow
            label={day.absence.label}
            wording={day.absence.wording}
            note={day.absence.note}
            variant="cell"
          />
        );
      }
      return <ArrivalTimeRow {...day.arrival} variant="cell" />;
    },
    // Short form under the name in the phone list (#3834).
    stackedRender: (student) => {
      if (!canSeeDay(student)) return EMPTY;
      const day = getDay(student);
      if (day.absence) {
        return (
          <StudentAbsenceRow
            label={day.absence.label}
            wording={day.absence.wording}
            variant="compact"
          />
        );
      }
      return <ArrivalTimeRow {...day.arrival} variant="compact" />;
    },
    // Planned time; children without one sort last.
    sortValue: (student) =>
      canSeeDay(student) && !getDay(student).absence
        ? (getDay(student).arrival.arrivalTime ?? "~")
        : "~",
  };
}

export function pickupColumn<T extends Student>(
  getDay: (student: T) => StudentTableDay,
): StudentTableColumn<T> {
  return {
    key: "pickup",
    header: "Gehzeit",
    // Short form under the name in the phone list (#3834).
    stackedRender: (student) => {
      if (!canSeeDay(student)) return EMPTY;
      const day = getDay(student);
      if (day.absence) return EMPTY;
      return <PickupTimeRow {...day.pickup} variant="compact" />;
    },
    render: (student) => {
      if (!canSeeDay(student)) return EMPTY;
      const day = getDay(student);
      // An absent child has no pickup; the dash keeps the row calm.
      if (day.absence) return EMPTY;
      return <PickupTimeRow {...day.pickup} variant="cell" />;
    },
    sortValue: (student) =>
      canSeeDay(student) && !getDay(student).absence
        ? (getDay(student).pickup.pickupTime ?? "~")
        : "~",
  };
}

export function departureColumn<T extends Student>(
  labelFor: (student: T) => string,
): StudentTableColumn<T> {
  return {
    key: "departure",
    header: "Heimweg",
    render: (student) =>
      canSeeDay(student) ? (
        <span className="text-sm text-gray-700">{labelFor(student)}</span>
      ) : (
        EMPTY
      ),
  };
}

/** Only offered when the school tracks something; returns null otherwise. */
export function trackingColumn<T extends Student>(
  trackingData: TrackingIndicatorsResponse | undefined,
  enabled = true,
): StudentTableColumn<T> | null {
  if (!enabled || !trackingData?.labels?.length) return null;
  return {
    key: "tracking",
    header: "Erledigt",
    render: (student) =>
      canSeeDay(student) ? (
        <TrackingIndicators
          labels={trackingData.labels}
          results={trackingData.results[student.id] ?? []}
          layout="inline"
        />
      ) : (
        EMPTY
      ),
  };
}

/** Drops the columns a page does not offer right now (null entries). */
export function compactColumns<T>(
  columns: readonly (StudentTableColumn<T> | null | false | undefined)[],
): StudentTableColumn<T>[] {
  return columns.filter((column): column is StudentTableColumn<T> =>
    Boolean(column),
  );
}

/** Column defaults for useCollectionView, from the page's column list. */
export function columnDefaults<T>(columns: readonly StudentTableColumn<T>[]) {
  return columns
    .filter((column) => column.hideable !== false)
    .map((column) => ({
      id: column.key,
      defaultVisible: column.defaultVisible ?? true,
    }));
}

interface StudentTableProps<T extends Student> {
  readonly rows: readonly T[];
  readonly columns: StudentTableColumn<T>[];
  readonly hiddenColumns: ReadonlySet<string>;
  readonly onOpen: (student: T) => void;
  /** Marked children; omit to show the table without checkboxes. */
  readonly selection?: {
    readonly selectedIds: ReadonlySet<string>;
    readonly onChange: (ids: readonly string[], selected: boolean) => void;
    readonly disabled?: boolean;
  };
  /** Heading above the table, e.g. a grouping label with its count. */
  readonly caption?: string;
  /** Column shown under each name in the phone list; null for none. */
  readonly phoneDetail?: string | null;
}

/**
 * One table of children (#3834). Rows open the child; the checkbox column
 * marks children for the selection bar above.
 */
export function StudentTable<T extends Student>({
  rows,
  columns,
  hiddenColumns,
  onOpen,
  selection,
  caption,
  phoneDetail = null,
}: StudentTableProps<T>) {
  return (
    <DataTable<T>
      columns={columns}
      rows={[...rows]}
      getRowKey={(student) => student.id.toString()}
      onRowClick={onOpen}
      hiddenColumns={hiddenColumns}
      caption={caption}
      // On a phone one line per child (#3834): name and status, plus the
      // one detail the user chose under the name ("In der Zeile zeigen").
      stackedOnMobile
      stackedLayout="row"
      stackedDetailKey={phoneDetail}
      selection={
        selection
          ? {
              selectedKeys: selection.selectedIds,
              onChange: selection.onChange,
              rowLabel: studentFullName,
              disabled: selection.disabled,
            }
          : undefined
      }
    />
  );
}
