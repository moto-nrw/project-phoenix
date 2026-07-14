"use client";

import { CalendarClock, MapPin } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { InfoCard } from "~/components/ui/info-card";
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
      <div className="flex flex-wrap items-center gap-1.5">
        {block.isSubstitute && <Badge tone="blue">Vertretung</Badge>}
        {block.cancelled && (
          <Badge tone="red">
            {block.cancelReason
              ? `Entfällt · ${block.cancelReason}`
              : "Entfällt"}
          </Badge>
        )}
        {block.isAbsent && (
          <Badge tone="amber">
            {block.absenceReason
              ? `Abwesend · ${block.absenceReason}`
              : "Abwesend"}
          </Badge>
        )}
        {block.understaffedAck && !block.cancelled && (
          <Badge tone="orange">Unterbesetzt</Badge>
        )}
      </div>
    </li>
  );
}

// Tone maps to the brand palette (LOCATION_COLORS) via arbitrary-value hex, so
// the badges read like the rest of the app rather than generic Tailwind hues.
const TONE_CLASS: Record<"blue" | "red" | "amber" | "orange", string> = {
  blue: "bg-[#5080D8]/10 text-[#5080D8]",
  red: "bg-[#FF3130]/10 text-[#FF3130]",
  amber: "bg-[#EAB308]/10 text-[#A16207]",
  orange: "bg-[#F78C10]/10 text-[#F78C10]",
};

function Badge({
  tone,
  children,
}: {
  readonly tone: "blue" | "red" | "amber" | "orange";
  readonly children: React.ReactNode;
}) {
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${TONE_CLASS[tone]}`}
    >
      {children}
    </span>
  );
}
