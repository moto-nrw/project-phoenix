"use client";

import { CalendarClock, MapPin } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { InfoCard } from "~/components/ui/info-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { ownShiftService } from "~/lib/shift-api";
import type { OwnAssignment } from "~/lib/shift-helpers";
import { useSWRAuth } from "~/lib/swr";

// Heute geplante Betreuungsplan-Einsätze der eingeloggten Person ("Mein Tag",
// #1844): Ort (Raum) + Aufgabe (Aktivität) + Zeit, plus die Vertretungsplan-
// Zustände (Vertretung, entfällt, du fehlst, unterbesetzt). Bewusst ohne
// Kindernamen (GDPR). Rendert nichts, wenn die Schule keinen Betreuungsplan
// pflegt (leere Liste), damit die Seite nicht mit einer leeren Karte zusteht.
// Ein Ladefehler zeigt dagegen eine Fehlerkarte, damit er nicht wie "keine
// Einsätze" aussieht.
export function BetreuungsplanHeuteCard() {
  // Berlin, not browser-local: the backend defines "today" in Europe/Berlin,
  // and a browser in another timezone around midnight would otherwise fetch
  // yesterday's/tomorrow's assignments and label them "Heute geplant".
  const today = useBerlinToday();
  const { data: assignments, error } = useSWRAuth<OwnAssignment[]>(
    `time-tracking-own-assignments-today-${today}`,
    () => ownShiftService.getOwnAssignments(today, today),
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );

  // A fetch failure must stay distinguishable from "keine Einsätze geplant":
  // with only one retry and focus revalidation off, silently rendering the
  // empty-state (null) would hide the employee's schedule until a reload.
  if (error) {
    return (
      <InfoCard
        title="Heute geplant"
        icon={<CalendarClock className="h-5 w-5" />}
      >
        <Alert
          type="error"
          message="Die heutigen Einsätze konnten nicht geladen werden. Bitte die Seite neu laden."
        />
      </InfoCard>
    );
  }

  const blocks = (assignments ?? [])
    .filter((a) => a.date === today)
    .slice()
    .sort((a, b) => a.startTime.localeCompare(b.startTime));

  if (blocks.length === 0) return null;

  return (
    <InfoCard
      title="Heute geplant"
      icon={<CalendarClock className="h-5 w-5" />}
    >
      <ul className="divide-y divide-gray-100">
        {blocks.map((block) => (
          <AssignmentRow key={block.instanceId} block={block} />
        ))}
      </ul>
    </InfoCard>
  );
}

function AssignmentRow({ block }: { readonly block: OwnAssignment }) {
  const dimmed = block.cancelled || block.isAbsent;
  return (
    <li className="flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:gap-4">
      <span
        className={`w-28 flex-shrink-0 text-sm font-medium tabular-nums ${
          dimmed ? "text-gray-400 line-through" : "text-gray-900"
        }`}
      >
        {block.startTime}–{block.endTime}
      </span>
      <div className="min-w-0 flex-1">
        <p
          className={`truncate text-sm font-medium ${
            dimmed ? "text-gray-400" : "text-gray-900"
          }`}
        >
          {block.title}
          {block.groupName && block.groupName !== block.title && (
            <span className="font-normal text-gray-500">
              {" "}
              · {block.groupName}
            </span>
          )}
        </p>
        {block.roomName && (
          <p className="mt-0.5 flex items-center gap-1 text-xs text-gray-500">
            <MapPin className="h-3.5 w-3.5 flex-shrink-0" />
            {block.roomName}
          </p>
        )}
      </div>
      {/* Kit StatusBadge — the local Badge copy carried its own tone map of the
          same brand hexes. Abwesend and Unterbesetzt both land on the orange
          tone; they never appear on the same block. */}
      <div className="flex flex-wrap items-center gap-1.5">
        {block.isSubstitute && <StatusBadge tone="blue" label="Vertretung" />}
        {block.cancelled && (
          <StatusBadge
            tone="red"
            label={
              block.cancelReason
                ? `Entfällt · ${block.cancelReason}`
                : "Entfällt"
            }
          />
        )}
        {block.isAbsent && (
          <StatusBadge
            tone="orange"
            label={
              block.absenceReason
                ? `Abwesend · ${block.absenceReason}`
                : "Abwesend"
            }
          />
        )}
        {block.understaffedAck && !block.cancelled && (
          <StatusBadge tone="orange" label="Unterbesetzt" />
        )}
      </div>
    </li>
  );
}
