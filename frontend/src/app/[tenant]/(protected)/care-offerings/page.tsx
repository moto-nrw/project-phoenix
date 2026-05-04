import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";

export default function CareOfferingsPage() {
  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">
          Betreuungsangebote
        </h1>
        <p className="mt-1 text-sm text-gray-600">
          Verwalte den Katalog der Betreuungsangebote, aus dem Eltern bei der
          Anmeldung wählen können (z. B. Regelbetreuung, Mensa). Jede
          Anmeldephase hat ihren eigenen Katalog; Angebote lassen sich einfach
          in eine andere Phase klonen.
        </p>
      </header>
      <CareOfferingsEditor />
    </div>
  );
}
