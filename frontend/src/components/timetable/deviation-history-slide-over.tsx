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

function eventDescription(ev: DeviationHistoryEvent): string {
  const subject =
    ev.subjectStaffName ?? (ev.subjectStaffId ? "Unbekannte Person" : null);
  const related =
    ev.relatedStaffName ?? (ev.relatedStaffId ? "Unbekannte Person" : null);
  switch (ev.eventType) {
    case "absence":
      return subject
        ? `${subject} wurde als abwesend eingetragen.`
        : "Eine Abwesenheit wurde eingetragen.";
    case "return_to_presence":
      return subject
        ? `${subject} wurde wieder als anwesend eingetragen.`
        : "Eine Abwesenheit wurde zurückgenommen.";
    case "substitution":
      if (subject && related) return `${related} vertritt ${subject}.`;
      return "Eine Vertretung wurde zugewiesen.";
    case "substitute_removed":
      return subject
        ? `Die Vertretung durch ${subject} wurde entfernt.`
        : "Eine Vertretung wurde entfernt.";
    case "cancellation":
      return "Der Block wurde abgesagt.";
    case "understaffed_ack":
      return "Die offene Besetzung wurde bewusst akzeptiert.";
    case "understaffed_unack":
      return "Die Kennzeichnung als bewusst unbesetzt wurde aufgehoben.";
    case "deviation_dropped_by_replan":
      return "Eine Neuplanung hat die eingetragenen Abweichungen dieses Termins verworfen.";
    case "deviation_dropped_by_edit":
      return subject
        ? `Eine Bearbeitung des Termins hat die Abweichung von ${subject} verworfen.`
        : "Eine Bearbeitung des Termins hat eingetragene Abweichungen verworfen.";
    case "sick_reported":
      return subject
        ? `${subject} wurde krank gemeldet.`
        : "Eine Krankmeldung wurde eingetragen.";
    case "sick_cleared":
      return subject
        ? `Die Krankmeldung von ${subject} wurde zurückgenommen.`
        : "Eine Krankmeldung wurde zurückgenommen.";
    default:
      return deviationEventLabel(ev.eventType);
  }
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
