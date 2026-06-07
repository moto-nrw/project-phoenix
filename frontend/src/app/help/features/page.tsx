import { GuideShell } from "~/components/help/guide-components";
import { appChapters } from "~/components/help/guide-data";

export const metadata = {
  title: "moto: Die App im Alltag",
  description:
    "Jeder Bereich der moto-App verständlich erklärt: was er macht und wie man ihn nutzt.",
};

export default function FeaturesGuidePage() {
  return (
    <GuideShell
      eyebrow="Die App im Alltag"
      title="Jeder Bereich der App, erklärt."
      description="Diese Anleitung erklärt die wichtigsten Bereiche der App. Zu jedem Bereich steht hier, was er macht und wie Sie ihn im Alltag nutzen."
      chapters={appChapters}
      activePath="funktionen"
      numbered={false}
    />
  );
}
