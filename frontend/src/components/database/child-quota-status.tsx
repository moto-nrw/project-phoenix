import { Info } from "lucide-react";
import { Tooltip } from "~/components/ui/tooltip";
import { formatCount } from "~/lib/format-utils";
import type { ChildQuota } from "~/lib/child-quota-api";

function explanation(quota: ChildQuota): string {
  const booked = `${formatCount(quota.booked)} ${quota.booked === 1 ? "Kind" : "Kinder"}`;
  const text = `Ihr Vertrag erlaubt bis zu ${booked}. Es zählen aktive Kinder und Kinder, deren Betreuung später beginnt.`;
  return quota.occupied >= quota.booked
    ? `${text} Für weitere Kinder melden Sie sich bitte beim moto-Team.`
    : text;
}

/**
 * Kinderkontingent der Schule in der Statuszeile der Kinderliste (#3569). Die
 * Zahl steht mit Namen in einer eigenen Zeile, damit sie nicht mit der Zahl
 * der Kinder in der Liste verwechselt wird: die Kontingentzahl zählt auch
 * Kinder, deren Betreuung erst später beginnt. Die Statuszeile trägt nur
 * Zahlen (frontend-ui-kit), die Erklärung steht im Tooltip am Info-Symbol.
 * Sie steht im Absatz der Statuszeile, deshalb nur Spans.
 */
export function ChildQuotaStatus({
  quota,
}: Readonly<{ quota: ChildQuota | null }>) {
  if (!quota) return null;
  const full = quota.occupied >= quota.booked;
  return (
    // Die Blase hängt an der ganzen Zeile, nicht am Symbol: auf dem Telefon
    // bricht die Zeile um und eine Blase am Symbol liefe aus dem Bildschirm.
    <span className="relative mt-1 block">
      Kinderkontingent:{" "}
      <span className="font-medium text-gray-900 tabular-nums">
        {`${formatCount(quota.occupied)} von ${formatCount(quota.booked)}`}
      </span>{" "}
      belegt{full ? " · voll" : ""}{" "}
      <Tooltip
        content={explanation(quota)}
        className="static align-text-bottom"
        bubbleClassName="right-0 w-auto max-w-sm"
      >
        <Info aria-label="Was zählt?" className="h-4 w-4 text-gray-500" />
      </Tooltip>
    </span>
  );
}
