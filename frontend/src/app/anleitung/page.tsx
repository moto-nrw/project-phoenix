import Link from "next/link";
import { ArrowRight, MonitorSmartphone, TabletSmartphone } from "lucide-react";

export const metadata = {
  title: "moto Anleitung",
  description: "Schritt für Schritt Anleitung für moto.",
};

export default function AnleitungLandingPage() {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen overflow-x-hidden">
      <div className="relative mx-auto flex min-h-screen w-full max-w-5xl flex-col px-4 py-5 sm:px-6 lg:px-8">
        <header className="flex items-center justify-between rounded-3xl border border-gray-200 bg-white/90 px-4 py-3 shadow-sm backdrop-blur-md">
          <Link
            href="/"
            className="flex items-center gap-3 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <span className="flex h-10 w-10 items-center justify-center rounded-2xl border border-gray-200 bg-white text-sm font-bold text-gray-950 shadow-sm">
              m
            </span>
            <span>
              <span className="block text-sm font-semibold text-gray-950">
                moto
              </span>
              <span className="block text-xs text-gray-500">Anleitung</span>
            </span>
          </Link>
        </header>

        <section className="flex flex-1 items-center py-8 sm:py-10">
          <div className="w-full">
            <div className="mx-auto max-w-2xl text-center">
              <p className="text-sm font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
                Anleitung öffnen
              </p>
              <h1 className="mt-3 text-4xl leading-tight font-semibold tracking-normal text-gray-950 sm:text-5xl">
                Welche Version von moto nutzen Sie?
              </h1>
              <p className="mt-4 text-base leading-7 text-gray-600">
                Wählen Sie eine Variante. Danach öffnet sich die passende
                Anleitung mit PDF-Funktion.
              </p>
            </div>

            <div className="mt-8 grid gap-4 lg:grid-cols-2">
              <SelectionCard
                href="/anleitung/standard"
                title="moto Standard"
                body="Für Einrichtungen ohne NFC-Tablets."
                icon={MonitorSmartphone}
                points={[
                  "Kinder, Mitarbeitende, Räume und Gruppen",
                  "Stundenplan und spontane Aktivitäten",
                  "Zeiterfassung, Anmeldungen und Feedback",
                ]}
              />
              <SelectionCard
                href="/anleitung/nfc"
                title="moto NFC"
                body="Für Einrichtungen mit NFC-Tablets oder NFC-Armbändern."
                icon={TabletSmartphone}
                points={[
                  "Alles aus moto Standard",
                  "Vorbereitung für Armbänder",
                  "Aktivitäten, Räume, Gruppen und Geräte",
                ]}
              />
            </div>

            <p className="mt-6 text-center text-sm leading-6 text-gray-500">
              Unsicher? Starten Sie mit `moto Standard`. Die NFC-Anleitung kann
              später jederzeit geöffnet werden.
            </p>
          </div>
        </section>
      </div>
    </main>
  );
}

function SelectionCard({
  href,
  title,
  body,
  icon: Icon,
  points,
}: {
  readonly href: string;
  readonly title: string;
  readonly body: string;
  readonly icon: typeof MonitorSmartphone;
  readonly points: readonly string[];
}) {
  return (
    <Link
      href={href}
      className="group moto-content-surface rounded-3xl border p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:p-6"
    >
      <div className="flex items-start justify-between gap-4">
        <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-[#83CD2D]/14 text-[#3F6F12]">
          <Icon className="h-6 w-6" aria-hidden="true" />
        </span>
        <ArrowRight
          className="h-5 w-5 text-gray-400 transition-transform group-hover:translate-x-1"
          aria-hidden="true"
        />
      </div>
      <h2 className="mt-5 text-2xl font-semibold tracking-normal text-gray-950">
        {title}
      </h2>
      <p className="mt-2 text-sm leading-6 text-gray-600">{body}</p>
      <ul className="mt-5 space-y-2 text-sm leading-6 text-gray-700">
        {points.map((point) => (
          <li key={point} className="flex gap-2">
            <span
              className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-[#83CD2D]"
              aria-hidden="true"
            />
            <span>{point}</span>
          </li>
        ))}
      </ul>
    </Link>
  );
}
