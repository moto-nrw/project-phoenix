"use client";

import { SectionCard } from "~/components/ui/section-card";

interface DesktopOnlyNoticeProps {
  readonly title?: string;
  /**
   * Begründung, warum genau DIESE Seite den großen Screen braucht. Der
   * Standardtext spricht von der Anmeldungsverwaltung — wer die Komponente
   * anderswo einsetzt, muss ihn ersetzen, sonst steht dort eine falsche
   * Aussage (so geschehen auf der Planungsseite Kalenderzeiträume, #2033).
   */
  readonly description?: string;
}

export function DesktopOnlyNotice({
  title = "Bitte am Computer öffnen",
  description = "Diese Seite ist für die Arbeit am Computer optimiert. Bitte öffnen Sie sie auf einem Laptop oder Desktop-Rechner.",
}: DesktopOnlyNoticeProps) {
  return (
    // Auf einer Fläche, nicht auf dem gemusterten Grund: dort steht kein Text
    // (BAUARTEN-SPEC Teil 3). Der Hinweis ersetzt den Seiteninhalt und muss
    // deshalb dieselbe Fläche tragen wie der Inhalt, den er ersetzt.
    <SectionCard className="lg:hidden">
      <div className="flex min-h-[50vh] flex-col items-center justify-center px-2 text-center">
        <div className="bg-moto-blue/10 mb-6 rounded-full p-5">
          <svg
            className="text-moto-blue h-10 w-10"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={1.8}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
            />
          </svg>
        </div>
        <h2 className="mb-3 text-xl font-semibold text-gray-900">{title}</h2>
        <p className="max-w-md text-base text-gray-600">{description}</p>
      </div>
    </SectionCard>
  );
}
