/**
 * Shown when the tenant slug in the URL does not match any known school.
 */
export default function TenantNotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 p-4">
      <h1 className="text-2xl font-bold text-gray-900">
        Schule nicht gefunden
      </h1>
      <p className="text-gray-600">
        Die angegebene Schule existiert nicht oder ist nicht aktiv.
      </p>
      <p className="text-sm text-gray-400">
        Bitte überprüfen Sie die Subdomain in der Adressleiste.
      </p>
    </div>
  );
}
