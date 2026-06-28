"use client";

import { useMemo } from "react";
import {
  buildEnrollmentChangeRequestDiffGroups,
  formatEnrollmentChangeRequestValue,
  type EnrollmentChangeRequestDiffGroup,
} from "~/lib/enrollment-change-request-diff";
import { cn } from "~/lib/utils";

interface EnrollmentChangeRequestDiffProps {
  readonly baseSnapshot: Record<string, unknown>;
  readonly proposedSnapshot: Record<string, unknown>;
  readonly diff: Record<string, unknown>;
  readonly beforeLabel?: string;
  readonly afterLabel?: string;
  readonly emptyLabel?: string;
  readonly emptyMessage?: string;
  readonly className?: string;
}

export function EnrollmentChangeRequestDiff({
  baseSnapshot,
  proposedSnapshot,
  diff,
  beforeLabel = "Bisher",
  afterLabel = "Neu",
  emptyLabel = "Nicht gesetzt",
  emptyMessage = "Die Anfrage enthält keine erkennbaren Feldänderungen.",
  className,
}: EnrollmentChangeRequestDiffProps) {
  const groups = useMemo(
    () =>
      buildEnrollmentChangeRequestDiffGroups({
        baseSnapshot,
        proposedSnapshot,
        diff,
      }),
    [baseSnapshot, proposedSnapshot, diff],
  );

  if (groups.length === 0) {
    return <p className="text-sm text-gray-600">{emptyMessage}</p>;
  }

  return (
    <dl className={cn("space-y-3", className)}>
      {groups.map((group) => (
        <DiffGroup
          key={group.key}
          group={group}
          beforeLabel={beforeLabel}
          afterLabel={afterLabel}
          emptyLabel={emptyLabel}
        />
      ))}
    </dl>
  );
}

function DiffGroup({
  afterLabel,
  beforeLabel,
  emptyLabel,
  group,
}: {
  readonly group: EnrollmentChangeRequestDiffGroup;
  readonly beforeLabel: string;
  readonly afterLabel: string;
  readonly emptyLabel: string;
}) {
  return (
    <div className="grid gap-3 rounded-xl border border-gray-100 bg-gray-50/70 p-3 sm:grid-cols-[12rem_1fr]">
      <dt className="text-sm font-semibold text-gray-900">{group.label}</dt>
      <dd className="space-y-2 text-sm">
        {group.rows.map((row) => (
          <div key={row.id} className="rounded-lg bg-white px-3 py-2">
            <p className="text-xs font-medium break-words text-gray-500 uppercase">
              {row.label}
            </p>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              <DiffValue
                label={beforeLabel}
                value={row.before}
                emptyLabel={emptyLabel}
              />
              <DiffValue
                label={afterLabel}
                value={row.after}
                emptyLabel={emptyLabel}
              />
            </div>
          </div>
        ))}
      </dd>
    </div>
  );
}

function DiffValue({
  emptyLabel,
  label,
  value,
}: {
  readonly label: string;
  readonly value: unknown;
  readonly emptyLabel: string;
}) {
  return (
    <div className="rounded-lg bg-gray-50 px-3 py-2">
      <p className="text-xs font-medium text-gray-500 uppercase">{label}</p>
      <p className="mt-1 text-sm break-words text-gray-900">
        {formatEnrollmentChangeRequestValue(value, emptyLabel)}
      </p>
    </div>
  );
}
