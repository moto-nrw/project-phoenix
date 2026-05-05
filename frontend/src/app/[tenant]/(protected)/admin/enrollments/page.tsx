import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";

export default function AdminEnrollmentsPage() {
  return (
    <div className="mx-auto max-w-6xl space-y-6 p-6">
      <header>
        <h1 className="text-2xl font-semibold text-gray-900">Anmeldungen</h1>
        <p className="mt-1 text-sm text-gray-600">
          Übersicht aller eingegangenen Anmeldungen pro Phase. Klicke auf eine
          Anmeldung, um die Kinder zu prüfen und einzeln zu bestätigen, auf die
          Warteliste zu setzen oder abzulehnen.
        </p>
      </header>
      <AdminEnrollmentsList />
    </div>
  );
}
