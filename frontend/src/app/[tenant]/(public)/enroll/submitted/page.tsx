export default function EnrollSubmittedPage() {
  return (
    <main className="mx-auto w-full max-w-2xl p-4 sm:p-6">
      <div className="rounded-2xl border border-gray-100/50 bg-white/90 p-8 text-center shadow-sm backdrop-blur-md">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-[#83CD2D]/10">
          <svg
            className="h-6 w-6 text-[#5A8B1F]"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M5 13l4 4L19 7"
            />
          </svg>
        </div>
        <h1 className="text-2xl font-semibold text-gray-900">
          Anmeldung eingegangen
        </h1>
        <p className="mt-3 text-sm text-gray-600">
          Vielen Dank! Wir haben deine Anmeldung erfolgreich erhalten. Eine
          Bestätigungs-E-Mail mit dem Status-Link ist unterwegs zu der von dir
          angegebenen Adresse.
        </p>
        <p className="mt-3 text-sm text-gray-600">
          Über den Status-Link kannst du jederzeit den aktuellen Stand einsehen
          und solange noch keine Entscheidung getroffen wurde, dort auch
          Änderungen vornehmen oder einzelne Kinder zurückziehen.
        </p>
        <p className="mt-6 text-xs text-gray-500">
          Bitte überprüfe ggf. deinen Spam-Ordner, wenn die E-Mail nicht
          innerhalb weniger Minuten ankommt.
        </p>
      </div>
    </main>
  );
}
