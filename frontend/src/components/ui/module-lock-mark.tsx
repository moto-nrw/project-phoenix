import { LockSimpleIcon } from "@phosphor-icons/react/ssr";

/**
 * Das Schloss an einem Navigationseintrag eines Moduls, das die Schule nicht
 * gebucht hat (#4894). Nur wer die Schule leitet, sieht solche Einträge; der
 * Eintrag führt auf die Seite, und die erklärt, wer das Modul bucht.
 * Vorleseprogramme hören „nicht gebucht“ hinter dem Namen.
 *
 * `relative` hält den `sr-only`-Text (absolut positioniert) im Schloss. Ohne
 * Bezugsrahmen hängt er am Sticky-Rahmen der Seitenleiste, entgeht dem
 * Scrollbereich der Navigation und verlängert die ganze Seite.
 */
export function ModuleLockMark({
  className = "",
}: {
  readonly className?: string;
}) {
  return (
    <span
      className={`relative inline-flex shrink-0 items-center text-gray-400 ${className}`}
      title="Ihre Schule hat dieses Modul nicht gebucht"
    >
      <LockSimpleIcon size={14} weight="bold" aria-hidden="true" />
      <span className="sr-only">, nicht gebucht</span>
    </span>
  );
}
