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
      note="Die Stammdaten stehen jeweils an ihrer Sammlung: `Kinder`, `Mitarbeitende`, `Räume` und `Aktivitäten` haben dafür den Reiter `Stammdaten`. Gruppen pflegen Sie unter `Datenverwaltung` im Reiter `Gruppen`."
    />
  );
}
