"use client";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  otherOfferingsSentence,
  weekdayName,
  type OfferingPreview,
  type OfferingPreviewChild,
} from "~/lib/roster-deviation";

function names(children: readonly OfferingPreviewChild[]): string {
  return children.map((child) => child.name).join(", ");
}

/**
 * Who joins and who leaves a Regeltermin per weekday when it takes its
 * children from an offering (#5103). It names why children drop and, when
 * another offering of the phase books them, offers to add it: a school that
 * chose only „Randstunde“ saw its Ganztag children drop without a reason.
 */
export function OfferingPreviewList({
  preview,
  onAddOfferings,
}: Readonly<{
  preview: OfferingPreview;
  /** Adds the named offerings to the selection; without it, no button. */
  onAddOfferings?: (offeringIds: string[]) => void;
}>) {
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
  const anyRemoved = preview.weekdays.some((day) => day.removed.length > 0);
  const otherOfferings = preview.otherOfferings ?? [];
  const otherSentence = otherOfferingsSentence(otherOfferings);
  return (
    <div className="flex flex-col gap-2">
      {otherSentence ? (
        <Alert
          type="warning"
          announce="polite"
          message={otherSentence}
          actionLayout="stacked"
          action={
            onAddOfferings ? (
              <Button
                type="button"
                variant="surface"
                size="compact"
                onClick={() =>
                  onAddOfferings(
                    otherOfferings.map((offering) => offering.offeringId),
                  )
                }
              >
                {otherOfferings.length === 1
                  ? "Angebot dazunehmen"
                  : "Angebote dazunehmen"}
              </Button>
            ) : undefined
          }
        />
      ) : null}
      {anyRemoved ? (
        <p className="text-xs text-gray-600">
          Kinder fallen weg, wenn sie an diesem Tag in keinem gewählten Angebot
          angemeldet sind.
        </p>
      ) : null}
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
    </div>
  );
}
