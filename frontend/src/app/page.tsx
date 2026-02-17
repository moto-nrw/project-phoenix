/**
 * Root page for the bare domain (no subdomain).
 * Shown when a user visits e.g. localhost:3000 without a subdomain.
 * In production, this redirects to the main school subdomain.
 * During development, it shows a tenant selector.
 */
export default function RootPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 p-4">
      <h1 className="text-2xl font-bold text-gray-900">moto</h1>
      <p className="text-gray-600">
        Bitte verwenden Sie die Subdomain Ihrer Schule.
      </p>
      <div className="flex flex-col gap-3 text-center">
        <p className="text-sm text-gray-500">Entwicklung:</p>
        <a
          href="http://school-a.localhost:3000"
          className="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800"
        >
          school-a.localhost:3000
        </a>
        <a
          href="http://school-b.localhost:3000"
          className="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800"
        >
          school-b.localhost:3000
        </a>
      </div>
    </div>
  );
}
