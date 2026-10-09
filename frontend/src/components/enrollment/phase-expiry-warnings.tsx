"use client";

import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  listPhaseExpiryWarnings,
  type PhaseExpiryWarning,
} from "~/lib/enrollment-phase-api";
import { formatCalendarDate } from "~/lib/localized-date-format";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { cn } from "~/lib/utils";

interface PhaseExpiryWarningsProps {
  readonly onCreateSuccessor?: (sourcePhaseId: string) => void;
  readonly className?: string;
}

function childCount(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

function warningCopy(warning: PhaseExpiryWarning): {
  title: string;
  message: string;
  action: string;
} {
  const date = formatCalendarDate(warning.first_affected_date, "de-DE", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });

  if (warning.state === "missing_successor") {
    const count = childCount(warning.affected_children);
    return warning.overdue
      ? {
          title: "Buchungen fehlen",
          message: `Seit dem ${date} fehlen Buchungen für ${count}. Erstellen Sie jetzt eine Anschlussphase.`,
          action: "Anschlussphase erstellen",
        }
      : {
          title: "Anschlussphase fehlt",
          message: `Ab ${date} enden die Buchungen für ${count}. Erstellen Sie jetzt eine Anschlussphase.`,
          action: "Anschlussphase erstellen",
        };
  }

  const count = childCount(warning.unresolved_children);
  return warning.overdue
    ? {
        title: "Übernahme nicht abgeschlossen",
        message: `Seit dem ${date} fehlen Buchungen für ${count}. Schließen Sie jetzt die Übernahme ab.`,
        action: "Anschlussphase öffnen",
      }
    : {
        title: "Übernahme noch offen",
        message: `Ab ${date} fehlen noch Buchungen für ${count}. Schließen Sie jetzt die Übernahme ab.`,
        action: "Anschlussphase öffnen",
      };
}

function WarningAction({
  warning,
  label,
  onCreateSuccessor,
  tenantPath,
}: Readonly<{
  warning: PhaseExpiryWarning;
  label: string;
  onCreateSuccessor?: (sourcePhaseId: string) => void;
  tenantPath: (path: string) => string;
}>) {
  if (warning.state === "missing_successor" && onCreateSuccessor) {
    return (
      <Button
        type="button"
        variant="surface"
        size="md"
        onClick={() => onCreateSuccessor(warning.source_phase_id)}
      >
        {label}
      </Button>
    );
  }

  let href: string;
  if (warning.state === "missing_successor") {
    href = tenantPath(
      `/enrollment-phases?rollover=${encodeURIComponent(warning.source_phase_id)}`,
    );
    const mobileHref = tenantPath(
      `/enrollment-phases/${encodeURIComponent(warning.source_phase_id)}/rollover`,
    );
    return (
      <>
        <ButtonLink
          href={mobileHref}
          variant="surface"
          size="md"
          className="lg:hidden"
        >
          {label}
        </ButtonLink>
        <ButtonLink
          href={href}
          variant="surface"
          size="md"
          className="hidden lg:inline-flex"
        >
          {label}
        </ButtonLink>
      </>
    );
  } else {
    const successorPhaseID = warning.successor_phase_id;
    if (!successorPhaseID) return null;
    href = tenantPath(
      `/admin/enrollments/phases/${encodeURIComponent(successorPhaseID)}`,
    );
  }
  return (
    <ButtonLink href={href} variant="surface" size="md">
      {label}
    </ButtonLink>
  );
}

export function PhaseExpiryWarnings({
  onCreateSuccessor,
  className,
}: Readonly<PhaseExpiryWarningsProps>) {
  const tenantPath = useTenantAwarePath();
  const { data, error, mutate } = useSWRAuth(
    "enrollment-phase-expiry-warnings",
    listPhaseExpiryWarnings,
  );
  // Die Hinweise liest das Backend mit config:manage, dem Recht, das die
  // erfragte Folgephase anlegt (#3469). Eine Rolle, die Anmeldungen ohne
  // dieses Recht bearbeitet, bekommt hier 403: dann gibt es bewusst keinen
  // Hinweis und keinen Fehler statt einer Fehlerkarte, die sie nicht
  // auflösen kann.
  const forbidden = (error as { status?: number } | undefined)?.status === 403;
  const loadError = useSwrLoadError(
    forbidden ? undefined : error,
    "die Übersicht zum Phasenende",
    () => mutate(),
  );

  if (error) {
    if (forbidden) return null;
    return <LoadErrorAlert error={loadError} className={className} />;
  }
  if (!Array.isArray(data) || data.length === 0) return null;

  return (
    <div className={cn("space-y-3", className)}>
      {data.map((warning) => {
        const copy = warningCopy(warning);
        return (
          <Alert
            key={warning.source_phase_id}
            type={warning.overdue ? "error" : "warning"}
            announce={warning.overdue ? "assertive" : "polite"}
            title={copy.title}
            message={copy.message}
            action={
              <WarningAction
                warning={warning}
                label={copy.action}
                onCreateSuccessor={onCreateSuccessor}
                tenantPath={tenantPath}
              />
            }
          />
        );
      })}
    </div>
  );
}
