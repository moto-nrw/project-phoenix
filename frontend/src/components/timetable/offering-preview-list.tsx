"use client";

import { FramedList } from "~/components/ui/framed-list";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  droppedChildReason,
  otherOfferingsSentence,
  weekdayName,
  type OfferingPreview,
  type OfferingPreviewChild,
  type OfferingPreviewDroppedChild,
} from "~/lib/roster-deviation";

function names(children: readonly OfferingPreviewChild[]): string {
  return children.map((child) => child.name).join(", ");
}

/**
 * The children who drop on one weekday, each with why: Schule am Berg
 * connected all offerings and still saw a child drop that was booked on
 * Monday only, while the Regeltermin runs on other days.
 */
function DroppedChildren({
  dropped,
}: Readonly<{ dropped: readonly OfferingPreviewDroppedChild[] }>) {
  return (
    <div className="mt-1.5">
      <p className="text-gray-700">{`Fällt weg (${dropped.length})`}</p>
      <FramedList className="mt-1">
        {dropped.map((child) => {
          const reason = droppedChildReason(child);
          return (
            <li
              key={child.studentId}
              className="flex flex-wrap items-baseline justify-between gap-x-3 px-3 py-2"
            >
              <span className="text-gray-900">{child.name}</span>
              {reason ? (
                <span className="text-xs text-gray-500">{reason}</span>
              ) : null}
            </li>
          );
        })}
      </FramedList>
    </div>
  );
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
  disabled = false,
}: Readonly<{
  preview: OfferingPreview;
  /** Adds the named offerings to the selection; without it, no button. */
  onAddOfferings?: (offeringIds: string[]) => void;
  disabled?: boolean;
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
                disabled={disabled}
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
          angemeldet sind. Soll ein Kind trotzdem kommen, ändern Sie seine
          Angebote in der Anmeldung.
        </p>
      ) : null}
      <ul className="flex flex-col gap-2">
        {preview.weekdays.map((day) => (
          <li
            key={day.weekday}
            className="rounded-lg border border-gray-200 px-3 py-2 text-sm"
          >
            <div className="flex items-baseline justify-between gap-3">
              <p className="font-medium text-gray-900">
                {weekdayName(day.weekday)}
              </p>
              <p className="text-gray-500">{`Bleibt (${day.keptCount})`}</p>
            </div>
            {day.added.length > 0 ? (
              <p className="mt-0.5 text-gray-700">
                {`Kommt dazu (${day.added.length}): ${names(day.added)}`}
              </p>
            ) : null}
            {day.removed.length > 0 ? (
              <DroppedChildren dropped={day.removed} />
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
