import { GuideShell } from "~/components/help/guide-components";
import { setupChapters } from "~/components/help/guide-data";

export const metadata = {
  title: "moto Ersteinrichtung",
  description:
    "Schritt-für-Schritt-Anleitung für das erste Einrichten von moto.",
};

export default function SetupGuidePage() {
  return (
    <GuideShell
      eyebrow="Ersteinrichtung"
      title="moto Schritt für Schritt einrichten."
      description="Arbeiten Sie die Schritte von oben nach unten ab. Die Reihenfolge ist bewusst gewählt. Jeder Schritt baut auf dem vorherigen auf, vom Zugang bis zum Testlauf vor dem ersten echten Betreuungstag."
      chapters={setupChapters}
      activePath="ersteinrichtung"
      numbered
      note="Fast alle diese Schritte erledigen Sie zentral unter `Datenverwaltung` in der Seitenleiste. Dort werden Personal, Räume, Gruppen, Aktivitäten und Kinder gepflegt."
    />
  );
}
