import type { Metadata } from "next";
import { Suspense } from "react";
import { HelpView } from "~/components/help/help-view";

// Optionaler Catch-all: `/help` zeigt den Einstieg, `/help/<thema>` eine
// Anleitung, `/help/gruppe/<id>` eine Oberkategorie. Die Adressen sind flach,
// damit ein Thema beim Umsortieren seine Adresse behaelt
// (docs/hilfebereich-umbau-plan.md, 7.1).
//
// Die statische Druckseite `/help/nfc/erste-schritte` liegt daneben und
// gewinnt gegen diesen Catch-all, weil Next.js konkrete Segmente zuerst
// prueft.
export const metadata: Metadata = {
  title: "moto Hilfe",
  description:
    "Anleitungen für moto: für Betreuungskräfte, die Leitung, Eltern und Lehrkräfte.",
};

export default function HelpPage() {
  return (
    <Suspense fallback={<div className="min-h-screen bg-gray-50" />}>
      <HelpView />
    </Suspense>
  );
}
