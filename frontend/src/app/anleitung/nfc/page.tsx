import { Radio, Search, ShieldCheck } from "lucide-react";
import {
  GuideShell,
  LandingFeatureCard,
} from "~/components/anleitung/guide-components";
import { nfcChapters, nfcHighlights } from "~/components/anleitung/guide-data";

export const metadata = {
  title: "moto NFC Anleitung",
  description: "Anleitung für moto mit zusätzlichen NFC-Kapiteln.",
};

export default function NfcGuidePage() {
  return (
    <GuideShell
      eyebrow="moto NFC"
      title="Standard-Anleitung plus Vorbereitung für NFC."
      description="Diese Anleitung enthält alle Standard-Inhalte und zusätzliche Kapitel für Einrichtungen mit NFC-Tablets oder NFC-Armbändern."
      chapters={nfcChapters}
      activeVariant="nfc"
    >
      <div className="mt-7 grid gap-3 sm:grid-cols-3">
        {nfcHighlights.map((item) => (
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
          title="Erst moto prüfen"
          body="Kinder, Gruppen, Räume und Aktivitäten sollten vollständig vorbereitet sein."
          icon={Search}
        />
        <LandingFeatureCard
          title="Geräte im Blick"
          body="Der Bereich Geräte zeigt Status, Verbindung und letzte bekannte Informationen."
          icon={Radio}
        />
        <LandingFeatureCard
          title="Datenschutz klar halten"
          body="NFC-Armbänder enthalten keine Namen und keine persönlichen Daten."
          icon={ShieldCheck}
        />
      </div>
    </GuideShell>
  );
}
