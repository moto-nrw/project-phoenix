import { BrandLogo } from "~/components/dashboard/header/brand-link";

/**
 * Logo und Schriftmarke der Hilfe. „moto" steht in der Markenschrift wie im
 * Kopf der App, damit die Hilfe nicht wie eine fremde Seite wirkt.
 */
export function HelpWordmark() {
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <BrandLogo />
      <span className="truncate text-xl leading-tight font-bold text-gray-950">
        <span className="[font-family:var(--font-moto)]">moto</span>{" "}
        <span className="tracking-tight">Hilfe</span>
      </span>
    </span>
  );
}
