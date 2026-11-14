"use client";

import { ButtonLink } from "~/components/ui/button";
import {
  ResponsiveActionList,
  ResponsiveActionRow,
} from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  HolidayOwnChildAssignment,
  HolidayParticipant,
} from "~/lib/holiday-host-api";
import { useTenantAwarePath } from "~/lib/tenant-path";

/** Warum eine Anmeldung keinem Kind der OGS zugeordnet ist. */
function openReason(assignment: HolidayOwnChildAssignment): string {
  return assignment.assignment === "ambiguous"
    ? "Mehrere Kinder in moto haben diesen Namen und dieses Geburtsdatum."
    : "Kein Kind in moto hat diesen Namen und dieses Geburtsdatum.";
}

/** Was moto für die zugeordneten Kinder tut. */
function plannedStatus(matched: number): string {
  if (matched === 0) {
    return "Noch ist keine Anmeldung einem Kind zugeordnet.";
  }
  return matched === 1
    ? "1 Kind ist eingeplant. Der Check-in erwartet es an den gebuchten Tagen."
    : `${matched} Kinder sind eingeplant. Der Check-in erwartet sie an den gebuchten Tagen.`;
}

/**
 * Eigene Kinder in der Ferienwoche (#6678): Kinder dieser OGS, die für die
 * Woche angemeldet sind, bleiben in ihrer Gruppe. moto ordnet jede Anmeldung
 * über Vorname, Nachname und Geburtsdatum genau einem Kind zu und trägt dann
 * Ankunft und Abholung für die gebuchten Tage ein. Passt eine Anmeldung zu
 * keinem Kind oder zu mehreren, steht sie hier als offen, mit dem nächsten
 * Schritt. Ohne eigene Kinder in der Woche zeigt die Seite die Karte nicht.
 */
export function HolidayOwnChildrenCard({
  participants,
  assignments,
}: Readonly<{
  participants: readonly HolidayParticipant[];
  assignments: readonly HolidayOwnChildAssignment[];
}>) {
  const tenantPath = useTenantAwarePath();
  if (assignments.length === 0) return null;
  const byChild = new Map(
    participants.map((participant) => [participant.childId, participant]),
  );
  const matched = assignments.filter(
    (assignment) => assignment.assignment === "matched",
  ).length;
  const open = assignments
    .filter((assignment) => assignment.assignment !== "matched")
    .flatMap((assignment) => {
      const child = byChild.get(assignment.childId);
      return child ? [{ assignment, child }] : [];
    });

  return (
    <SectionCard
      title="Kinder Ihrer OGS"
      description="Diese Kinder bleiben in ihrer Gruppe. Notfall, Abholung und Zeiten aus der Anmeldung stehen in der Kinderakte."
      action={
        <div className="flex flex-wrap items-center justify-end gap-2">
          {matched > 0 ? (
            <StatusBadge tone="green" label={`${matched} eingeplant`} />
          ) : null}
          {open.length > 0 ? (
            <StatusBadge tone="warning" label={`${open.length} offen`} />
          ) : null}
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-sm text-gray-700">{plannedStatus(matched)}</p>
        {open.length > 0 && (
          <SectionCard
            title="Noch nicht zugeordnet"
            headingLevel={3}
            titleSize="sm"
            titleClassName="font-medium text-gray-600"
            bare
            className="space-y-3"
          >
            <p className="mb-3 text-sm text-gray-600">
              moto findet ein Kind über Vorname, Nachname und Geburtsdatum. Für
              diese Kinder ist nichts eingeplant. Ändern Sie die Kinderakte oder
              bitten Sie den Träger, die Anmeldung zu ändern.
            </p>
            <ResponsiveActionList>
              {open.map(({ assignment, child }) => {
                const name = `${child.firstName} ${child.lastName}`;
                return (
                  <ResponsiveActionRow
                    key={child.childId}
                    className="text-sm text-gray-700"
                  >
                    <div className="min-w-0">
                      <p>
                        <span className="font-medium text-gray-900">
                          {child.lastName}, {child.firstName}
                        </span>
                        , geboren am {formatDate(child.birthDate)}
                      </p>
                      <p className="text-gray-600">{openReason(assignment)}</p>
                    </div>
                    <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row min-[480px]:items-center sm:justify-end">
                      <ButtonLink
                        href={tenantPath(
                          `/students/search?search=${encodeURIComponent(child.lastName)}`,
                        )}
                        variant="surface"
                        size="compact"
                        aria-label={`Kind suchen: ${name}`}
                        tooltip="Öffnet die Kindersuche mit dem Nachnamen dieses Kindes."
                      >
                        Kind suchen
                      </ButtonLink>
                    </div>
                  </ResponsiveActionRow>
                );
              })}
            </ResponsiveActionList>
          </SectionCard>
        )}
      </div>
    </SectionCard>
  );
}
