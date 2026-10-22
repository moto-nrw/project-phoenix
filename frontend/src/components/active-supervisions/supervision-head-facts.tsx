import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

/**
 * Statuszeile der geöffneten Aufsicht (#4354): Kinder, Raum und wer die
 * Aufsicht hat, jeweils mit dem Symbol wie auf den Raumkarten unter „Räume“.
 * Raum und Betreuung fehlen, wenn es sie nicht gibt oder der Titel den Raum
 * schon nennt.
 */
export function SupervisionHeadFacts({
  childCount,
  roomName,
  supervisorNames,
}: Readonly<{
  /** „12 Kinder“, mit Grenze „12 / 10 Kinder · Überbucht“. */
  childCount: string;
  roomName: string | null;
  supervisorNames: readonly string[];
}>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-x-4 gap-y-1">
      <span className="inline-flex items-center gap-1">
        <MotoConceptIcon concept="children" size={16} />
        <span>{childCount}</span>
      </span>
      {roomName ? (
        <span className="inline-flex min-w-0 items-center gap-1">
          <MotoConceptIcon concept="rooms" size={16} />
          <span className="sr-only">Raum:</span>
          <span className="truncate">{roomName}</span>
        </span>
      ) : null}
      {supervisorNames.length > 0 ? (
        <span className="inline-flex min-w-0 items-center gap-1">
          <MotoConceptIcon concept="staff" size={16} />
          <span className="sr-only">Aufsicht:</span>
          <span>{supervisorNames.join(", ")}</span>
        </span>
      ) : null}
    </span>
  );
}
