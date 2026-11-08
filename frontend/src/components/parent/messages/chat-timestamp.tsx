/**
 * Zeitpunkt der letzten Nachricht in einer Gesprächszeile: am Telefon nur die
 * Uhrzeit, ab `sm` Datum und Uhrzeit. Die lange Form geht als `metaLabel` an
 * `ListRowLink` und wird vorgelesen.
 */
export function ChatTimestamp({
  full,
  compact,
}: Readonly<{ full: string; compact: string }>) {
  return (
    <>
      <span aria-hidden="true" className="sm:hidden">
        {compact}
      </span>
      <span aria-hidden="true" className="hidden sm:inline">
        {full}
      </span>
    </>
  );
}
