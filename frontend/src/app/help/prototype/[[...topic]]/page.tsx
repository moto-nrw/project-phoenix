import { Suspense } from "react";
import { HelpPrototype } from "~/components/help/prototype/help-prototype";

export const metadata = {
  title: "Prototyp: neuer moto Hilfebereich",
  description: "Klickbarer Prototyp für die neue Struktur der moto Hilfe.",
};

export default function HelpPrototypePage() {
  return (
    <Suspense fallback={<div className="min-h-screen bg-gray-50" />}>
      <HelpPrototype />
    </Suspense>
  );
}
