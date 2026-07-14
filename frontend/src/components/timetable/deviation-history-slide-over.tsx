"use client";

/**
 * DeviationHistorySlideOver — read-only Verlauf (Änderungsprotokoll, #1886)
 * for the Vertretungsplan: who changed which deviation when, with old→new
 * context and an optional reason. Two scopes: the selected block's slot or
 * the whole day.
 */

import { useMemo, useState } from "react";
import { History } from "lucide-react";

import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import { Loading } from "~/components/ui/loading";
import { formatDate } from "~/lib/date-helpers";
import { useSWRAuth } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import { deviationEventLabel } from "~/lib/timetable-helpers";
import { timetableMutedSurface } from "./timetable-style";
import type {
  DeviationHistoryEvent,
  EnrichedInstance,
} from "~/lib/timetable-types";

type HistoryScope = "block" | "day";

interface DeviationHistorySlideOverProps {
  instance: EnrichedInstance | null;
  open: boolean;
  onClose: () => void;
}

function formatOccurredAt(occurredAt: string): string {
  const d = new Date(occurredAt);
  if (Number.isNaN(d.getTime())) return occurredAt;
  return `${formatDate(occurredAt)}, ${d.toLocaleTimeString("de-DE", {
    hour: "2-digit",
    minute: "2-digit",
  })} Uhr`;
}

type EventDescription = (
  subject: string | null,
  related: string | null,
) => string;

const EVENT_DESCRIPTIONS: Readonly<Record<string, EventDescription>> = {
  absence: (subject) =>
    subject
      ? `${subject} wurde als abwesend eingetragen.`
      : "Eine Abwesenheit wurde eingetragen.",
  return_to_presence: (subject) =>
    subject
      ? `${subject} wurde wieder als anwesend eingetragen.`
      : "Eine Abwesenheit wurde zurückgenommen.",
  substitution: (subject, related) =>
    subject && related
      ? `${related} vertritt ${subject}.`
      : "Eine Vertretung wurde zugewiesen.",
  substitute_removed: (subject) =>
    subject
      ? `Die Vertretung durch ${subject} wurde entfernt.`
      : "Eine Vertretung wurde entfernt.",
  cancellation: () => "Der Block wurde abgesagt.",
  understaffed_ack: () => "Die offene Besetzung wurde bewusst akzeptiert.",
  understaffed_unack: () =>
    "Die Kennzeichnung als bewusst unbesetzt wurde aufgehoben.",
  deviation_dropped_by_replan: () =>
    "Eine Neuplanung hat die eingetragenen Abweichungen dieses Termins verworfen.",
  deviation_dropped_by_edit: (subject) =>
    subject
      ? `Eine Bearbeitung des Termins hat die Abweichung von ${subject} verworfen.`
      : "Eine Bearbeitung des Termins hat eingetragene Abweichungen verworfen.",
  sick_reported: (subject) =>
    subject
      ? `${subject} wurde krank gemeldet.`
      : "Eine Krankmeldung wurde eingetragen.",
  sick_cleared: (subject) =>
    subject
      ? `Die Krankmeldung von ${subject} wurde zurückgenommen.`
      : "Eine Krankmeldung wurde zurückgenommen.",
};

function staffName(name?: string, id?: string): string | null {
  return name ?? (id ? "Unbekannte Person" : null);
}

function eventDescription(ev: DeviationHistoryEvent): string {
  const describe = EVENT_DESCRIPTIONS[ev.eventType];
  if (!describe) return deviationEventLabel(ev.eventType);
  return describe(
    staffName(ev.subjectStaffName, ev.subjectStaffId),
    staffName(ev.relatedStaffName, ev.relatedStaffId),
  );
}

export function DeviationHistorySlideOver({
  instance,
  open,
  onClose,
}: DeviationHistorySlideOverProps) {
  const [scope, setScope] = useState<HistoryScope>("block");

  const date = instance?.date ?? null;
  const slotFiltered = scope === "block" && Boolean(instance?.activityGroupId);
  const swrKey =
    open && date
      ? `deviation-history-${date}-${slotFiltered ? `${instance?.activityGroupId}-${instance?.startTime}` : "day"}`
      : null;

  const { data, isLoading, error } = useSWRAuth(swrKey, () =>
    timetableService.getDeviationHistory(
      date!,
      date!,
      slotFiltered ? instance?.activityGroupId : undefined,
      slotFiltered ? instance?.startTime : undefined,
    ),
  );

  const events = useMemo(() => data?.events ?? [], [data]);

  return (
    <SlideOver open={open} onOpenChange={(o) => !o && onClose()}>
      <SlideOverContent>
        <SlideOverHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <SlideOverTitle className="flex items-center gap-2">
                <History className="h-5 w-5 text-gray-500" aria-hidden />
                Verlauf
              </SlideOverTitle>
              <SlideOverDescription>
                {instance
                  ? `${instance.title} · ${formatDate(instance.date)} · ${instance.startTime} Uhr`
                  : "Änderungsprotokoll"}
              </SlideOverDescription>
            </div>
            <SlideOverCloseButton />
          </div>
          {instance?.activityGroupId ? (
            <Tabs
              value={scope}
              onValueChange={(v) => setScope(v as HistoryScope)}
              className="mt-3"
            >
              <TabsList>
                <TabsTrigger value="block">Dieser Block</TabsTrigger>
                <TabsTrigger value="day">Ganzer Tag</TabsTrigger>
              </TabsList>
            </Tabs>
          ) : null}
        </SlideOverHeader>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {isLoading ? (
            <Loading />
          ) : error ? (
            <p className="text-sm text-gray-500">
              Der Verlauf konnte nicht geladen werden. Bitte erneut versuchen.
            </p>
          ) : events.length === 0 ? (
            <p className="text-sm text-gray-500">
              Für {scope === "block" ? "diesen Block" : "diesen Tag"} sind noch
              keine Änderungen protokolliert.
            </p>
          ) : (
            <ul className="space-y-3">
              {events.map((ev) => (
                <li
                  key={ev.id}
                  className={`${timetableMutedSurface} rounded-lg p-3`}
                >
                  <div className="flex items-baseline justify-between gap-2">
                    <span className="text-sm font-medium text-gray-900">
                      {deviationEventLabel(ev.eventType)}
                    </span>
                    <span className="shrink-0 text-xs text-gray-500">
                      {ev.startTime} Uhr
                    </span>
                  </div>
                  <p className="mt-1 text-sm text-gray-700">
                    {eventDescription(ev)}
                  </p>
                  {ev.reason ? (
                    <p className="mt-1 text-sm text-gray-500">
                      Begründung: {ev.reason}
                    </p>
                  ) : null}
                  <p className="mt-2 text-xs text-gray-500">
                    {formatOccurredAt(ev.occurredAt)}
                    {" · "}
                    {ev.actorName ?? "Unbekanntes Konto"}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </div>
      </SlideOverContent>
    </SlideOver>
  );
}
