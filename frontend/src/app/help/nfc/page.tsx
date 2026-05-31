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
      title="Zusätzliche Schritte für NFC-Einrichtungen."
      description="Nur für Einrichtungen mit Tablets oder NFC-Armbändern. Ersteinrichtung und die allgemeinen Funktionen gelten unverändert. Hier kommen ausschließlich die zusätzlichen NFC-Schritte dazu."
      chapters={nfcChapters}
      activePath="nfc"
      numbered
    />
  );
}
