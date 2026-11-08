"use client";

// Der Stand der Schreiben auf der Seite eines freigegebenen Monatslaufs
// (#4684): ein Satz, eine Zeile mit den Zahlen und der Weg zur Seite
// „Schreiben“. Erstellen, Senden, Drucken und Stornieren liegen dort, damit
// die Seite des Laufs deren Code nicht lädt.

import { ButtonLink } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import type { useApiLoadError } from "~/contexts/ToastContext";
import type { CareLetters } from "~/lib/traeger-care-billing-api";

/** „3 von 5 gesendet · 1 nicht angekommen · 1 ohne E-Mail · 1 storniert“. */
function careLettersStatusLine(letters: CareLetters): string {
  const { summary } = letters;
  const parts = [`${summary.sent} von ${summary.letters} gesendet`];
  if (summary.queued > 0) {
    parts.push(
      summary.queued === 1
        ? "1 wird gesendet"
        : `${summary.queued} werden gesendet`,
    );
  }
  if (summary.failed > 0) parts.push(`${summary.failed} nicht angekommen`);
  if (summary.withoutEmail > 0) {
    parts.push(`${summary.withoutEmail} ohne E-Mail`);
  }
  const cancelled = letters.documents.filter(
    (document) => document.kind === "cancellation",
  ).length;
  if (cancelled > 0) parts.push(`${cancelled} storniert`);
  return parts.join(" · ");
}

export function CareLettersSummaryCard({
  href,
  letters,
  loadError,
}: Readonly<{
  /** The page „Schreiben“ of this run. */
  href: string;
  /** null while it loads. */
  letters: CareLetters | null;
  loadError: ReturnType<typeof useApiLoadError>["error"];
}>) {
  const created = letters?.letters != null;
  return (
    <SectionCard
      title="Schreiben an die Familien"
      description="Jede Familie bekommt ihre Rechnung als Schreiben. Senden, Drucken und Stornieren liegen auf einer eigenen Seite."
      loading={!letters && !loadError}
      actions={
        <ButtonLink
          href={href}
          variant={created || loadError ? "outline" : "primary"}
          size="md"
        >
          {created || loadError ? "Schreiben öffnen" : "Schreiben erstellen"}
        </ButtonLink>
      }
    >
      {loadError ? (
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
      ) : letters ? (
        <p className="text-sm text-gray-700 tabular-nums">
          {created
            ? careLettersStatusLine(letters)
            : "Noch keine Schreiben erstellt."}
        </p>
      ) : null}
    </SectionCard>
  );
}
