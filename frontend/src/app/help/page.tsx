import Link from "next/link";
import { EntryPointCard } from "~/components/help/guide-components";
import { guideEntryPoints } from "~/components/help/guide-data";

export const metadata = {
  title: "moto Anleitung",
  description:
    "Anleitung für moto: Ersteinrichtung, alle Funktionen der App und NFC.",
};

export default function HelpLandingPage() {
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

        <section className="py-12 sm:py-16">
          <div className="max-w-2xl">
            <p className="text-sm font-bold tracking-[0.08em] text-[#3F6F12] uppercase">
              moto Handbuch
            </p>
            <h1 className="mt-3 text-4xl leading-tight font-semibold tracking-normal text-gray-950 sm:text-5xl">
              Wobei können wir helfen?
            </h1>
            <p className="mt-4 text-base leading-7 text-gray-600 sm:text-lg">
              Wählen Sie einen Bereich. Jede Anleitung führt Schritt für Schritt
              durch die Aufgabe und lässt sich als PDF speichern oder
              ausdrucken.
            </p>
          </div>

          <div className="mt-10 grid gap-5 md:grid-cols-3">
            {guideEntryPoints.map((entry) => (
              <EntryPointCard
                key={entry.href}
                href={entry.href}
                title={entry.title}
                body={entry.body}
                icon={entry.icon}
                points={entry.points}
              />
            ))}
          </div>
        </section>
      </div>
    </main>
  );
}
