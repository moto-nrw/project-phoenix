"use client";

import {
  weekdayName,
  type OfferingPreview,
  type OfferingPreviewChild,
} from "~/lib/roster-deviation";

function names(children: readonly OfferingPreviewChild[]): string {
  return children.map((child) => child.name).join(", ");
}

/**
 * Who joins and who leaves a Regeltermin per weekday when it takes its
 * children from an offering (#5103). Read-only: it only shows the effect of
 * the choice above it.
 */
export function OfferingPreviewList({
  preview,
}: Readonly<{ preview: OfferingPreview }>) {
  const changed = preview.weekdays.some(
    (day) => day.added.length > 0 || day.removed.length > 0,
  );
  if (!changed) {
    return (
      <p className="text-sm text-gray-600">
        An der Kinderliste ändert sich nichts.
      </p>
    );
  }
  return (
    <ul className="flex flex-col gap-2">
      {preview.weekdays.map((day) => (
        <li
          key={day.weekday}
          className="rounded-lg border border-gray-200 px-3 py-2 text-sm"
        >
          <p className="font-medium text-gray-900">
            {weekdayName(day.weekday)}
          </p>
          {day.added.length > 0 ? (
            <p className="mt-0.5 text-gray-700">
              {`Kommt dazu (${day.added.length}): ${names(day.added)}`}
            </p>
          ) : null}
          {day.removed.length > 0 ? (
            <p className="mt-0.5 text-gray-700">
              {`Fällt weg (${day.removed.length}): ${names(day.removed)}`}
            </p>
          ) : null}
          <p className="mt-0.5 text-gray-500">{`Bleibt (${day.keptCount})`}</p>
        </li>
      ))}
    </ul>
  );
}
