import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";

export default function EnrollmentFormPage() {
  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">
          Anmeldeformular bearbeiten
        </h1>
        <p className="mt-1 text-sm text-gray-600">
          Verwalte die benutzerdefinierten Felder, die Eltern beim Ausfüllen des
          Anmeldeformulars sehen. Kernfelder (Name, Geburtsdatum, Klassenstufe
          etc.) sind fest und werden hier nicht aufgeführt.
        </p>
      </header>
      <EnrollmentFormEditor />
    </div>
  );
}
