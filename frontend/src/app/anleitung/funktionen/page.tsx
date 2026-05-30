import { GuideShell } from "~/components/anleitung/guide-components";
import { appChapters } from "~/components/anleitung/guide-data";

export const metadata = {
  title: "moto: Die App im Alltag",
  description:
    "Jeder Punkt der moto-Seitenleiste verständlich erklärt: was er macht und wie man ihn nutzt.",
};

export default function FunktionenGuidePage() {
  return (
    <GuideShell
      eyebrow="Die App im Alltag"
      title="Jeder Punkt der Seitenleiste, erklärt."
      description="Diese Anleitung ist genau wie die Seitenleiste aufgebaut, von oben nach unten. Zu jedem Bereich steht hier, was er macht und wie Sie ihn im Alltag nutzen."
      chapters={appChapters}
      activePath="funktionen"
      numbered={false}
    />
  );
}
