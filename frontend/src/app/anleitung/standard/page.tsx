import { Clock3, Search, WandSparkles } from "lucide-react";
import {
  GuideShell,
  LandingFeatureCard,
} from "~/components/anleitung/guide-components";
import {
  standardChapters,
  standardHighlights,
} from "~/components/anleitung/guide-data";

export const metadata = {
  title: "moto Standard Anleitung",
  description: "Anleitung für moto ohne NFC-Tablets.",
};

export default function StandardGuidePage() {
  return (
    <GuideShell
      eyebrow="moto Standard"
      title="moto Schritt für Schritt einrichten und im Alltag nutzen."
      description="Diese Anleitung führt durch die wichtigsten Aufgaben ohne NFC-Tablet: Daten vorbereiten, Kinder suchen, Aufsicht führen, Stundenplan nutzen, Zeiten erfassen, Anmeldungen verwalten und Einstellungen prüfen."
      chapters={standardChapters}
      activeVariant="standard"
    >
      <div className="mt-7 grid gap-3 sm:grid-cols-3">
        {standardHighlights.map((item) => (
          <LandingFeatureCard
            key={item.title}
            title={item.title}
            body={item.body}
            icon={item.icon}
          />
        ))}
      </div>
      <div className="mt-5 grid gap-3 sm:grid-cols-3">
        <LandingFeatureCard
          title="Schnell suchen"
          body="Kindersuche und Filter bringen den Alltag schneller in die richtige Ansicht."
          icon={Search}
        />
        <LandingFeatureCard
          title="Spontan handeln"
          body="Aktivitäten können auch ohne vorbereiteten Termin direkt gestartet werden."
          icon={WandSparkles}
        />
        <LandingFeatureCard
          title="Zeiten dokumentieren"
          body="Arbeitszeit, Pausen, Homeoffice und Abwesenheiten werden an einem Ort gepflegt."
          icon={Clock3}
        />
      </div>
    </GuideShell>
  );
}
