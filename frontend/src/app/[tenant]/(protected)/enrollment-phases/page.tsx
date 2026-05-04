import { PhasesEditor } from "~/components/enrollment/phases-editor";

export default function EnrollmentPhasesPage() {
  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">Anmeldephasen</h1>
        <p className="mt-1 text-sm text-gray-600">
          Verwalte die einzelnen Anmeldephasen — z. B. ein Schuljahr, eine
          Ferienbetreuung oder ein Sonderzeitraum. Jede Phase hat ihren eigenen
          Anmeldezeitraum, ihr eigenes Formular und ihren eigenen
          Betreuungsangebote-Katalog. Eltern wählen vor dem Ausfüllen des
          Formulars eine Phase aus.
        </p>
      </header>
      <PhasesEditor />
    </div>
  );
}
