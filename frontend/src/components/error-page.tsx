import type { LucideIcon } from "lucide-react";
import { MotoBrand } from "~/components/auth/auth-shell";

/**
 * Ganzseitige Fehlerseite (Design: pen.dev-Entwurf zu #2624, Variante C):
 * offenes Layout ohne Karte, Markengrün-Akzente, dezente Deko-Kreise.
 * Eine Komponente für alle Textvarianten — Seite nicht gefunden, Schule
 * nicht gefunden, Funktion ausgeschaltet.
 *
 * Server-kompatibel; interaktive Aktionen kommen als Client-Children in den
 * `actions`-Slot.
 */
export function ErrorPage({
  visual,
  title,
  description,
  actions,
}: Readonly<{
  visual: React.ReactNode;
  title: string;
  description: string;
  actions?: React.ReactNode;
}>) {
  return (
    <main className="relative flex min-h-dvh flex-col items-center justify-center overflow-hidden bg-white px-4 py-16">
      <div
        aria-hidden="true"
        className="bg-moto-green-soft pointer-events-none absolute -top-44 -left-36 size-[26rem] rounded-full"
      />
      <div
        aria-hidden="true"
        className="bg-moto-green-soft pointer-events-none absolute -right-28 -bottom-32 size-[19rem] rounded-full"
      />
      <div className="relative flex flex-col items-center text-center">
        <MotoBrand />
        <div className="mt-8">{visual}</div>
        <h1 className="mt-8 text-3xl font-semibold text-gray-950 sm:text-4xl">
          {title}
        </h1>
        <p className="mt-3 max-w-md text-sm leading-6 text-gray-600 sm:text-base">
          {description}
        </p>
        {actions ? (
          <div className="mt-9 flex flex-wrap items-center justify-center gap-3">
            {actions}
          </div>
        ) : null}
      </div>
    </main>
  );
}

/** Große 404 im Stil der abgestimmten Variante C: die Null ist ein Ring in
 * Markengrün. Rein dekorativ — die Überschrift trägt die Information. */
export function ErrorPage404Visual() {
  return (
    <div aria-hidden="true" className="flex items-center">
      <span className="text-[6.5rem] leading-none font-extrabold text-gray-950 sm:text-[10rem]">
        4
      </span>
      <span className="border-moto-green mx-2 inline-block size-[4.6rem] rounded-full border-[1.05rem] sm:mx-3 sm:size-[7rem] sm:border-[1.6rem]" />
      <span className="text-[6.5rem] leading-none font-extrabold text-gray-950 sm:text-[10rem]">
        4
      </span>
    </div>
  );
}

/** Icon-Kreis in Markengrün für Textvarianten ohne 404-Zahl (Schule nicht
 * gefunden, Funktion ausgeschaltet). */
export function ErrorPageIconVisual({
  icon: Icon,
}: Readonly<{ icon: LucideIcon }>) {
  return (
    <div
      aria-hidden="true"
      className="bg-moto-green flex size-28 items-center justify-center rounded-full sm:size-32"
    >
      <Icon className="size-12 text-white sm:size-14" strokeWidth={1.75} />
    </div>
  );
}
