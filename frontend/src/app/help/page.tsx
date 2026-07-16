import { EntryPointCard, HelpHeader } from "~/components/help/guide-components";
import { guideEntryPoints } from "~/components/help/guide-data";
import { HelpSearchInline } from "~/components/help/help-search";

export const metadata = {
  title: "moto Anleitung",
  description:
    "Anleitung für moto: Ersteinrichtung, alle Funktionen der App und NFC.",
};

export default function HelpLandingPage() {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen overflow-x-hidden">
      <div className="relative mx-auto flex min-h-screen w-full max-w-5xl flex-col px-4 py-5 sm:px-6 lg:px-8">
        <HelpHeader />

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
              durch die Aufgabe und lässt sich als PDF herunterladen.
            </p>
          </div>

          <div className="mt-8">
            <HelpSearchInline />
          </div>

          <div className="mt-10 grid gap-5 md:grid-cols-2 xl:grid-cols-4">
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
