import { GuideShell } from "~/components/help/guide-components";
import { nfcChapters } from "~/components/help/guide-data";

export const metadata = {
  title: "moto NFC & Tablets",
  description:
    "Zusätzliche Schritte für Einrichtungen mit Tablets oder NFC-Armbändern.",
};

export default function NfcGuidePage() {
  return (
    <GuideShell
      eyebrow="NFC & Tablets"
      title="Das komplette Handbuch für das NFC-Tablet."
      description="Nur für Einrichtungen mit Tablets oder NFC-Armbändern. Von der Lieferung über das Aufstellen, Anmelden und Zuweisen der Armbänder bis zum täglichen Ein- und Auschecken und zur Fehlerbehebung – alles, was Ihr Team am Tablet braucht."
      chapters={nfcChapters}
      activePath="nfc"
      numbered
    />
  );
}
