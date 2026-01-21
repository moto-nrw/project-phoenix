import Link from "next/link";
import Image from "next/image";

export const metadata = {
  title: "Registrierung erfolgreich | moto",
  description: "Deine Organisation wird geprüft",
};

export default function SignupPendingPage() {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-gray-50 to-gray-100 px-4 py-12">
      <div className="w-full max-w-md space-y-8">
        {/* Logo */}
        <div className="flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="moto Logo"
            width={120}
            height={48}
            priority
          />
        </div>

        {/* Success Card */}
        <div className="rounded-2xl border border-gray-100 bg-white p-8 shadow-xl">
          {/* Success Icon */}
          <div className="mb-6 flex justify-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-green-100">
              <svg
                className="h-8 w-8 text-green-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M5 13l4 4L19 7"
                />
              </svg>
            </div>
          </div>

          <h1 className="mb-2 text-center text-2xl font-bold text-gray-900">
            Registrierung erfolgreich!
          </h1>
          <p className="mb-6 text-center text-gray-600">
            Vielen Dank für deine Registrierung bei moto.
          </p>

          {/* Status Info */}
          <div className="mb-6 rounded-xl border border-amber-100 bg-amber-50/50 p-4">
            <div className="flex items-start gap-3">
              <svg
                className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              </svg>
              <div>
                <p className="font-medium text-amber-800">
                  Deine Organisation wird geprüft
                </p>
                <p className="mt-1 text-sm text-amber-700">
                  Unser Team prüft deine Anfrage. Du erhältst eine E-Mail,
                  sobald deine Organisation freigeschaltet wurde.
                </p>
              </div>
            </div>
          </div>

          {/* Next Steps */}
          <div className="mb-6 space-y-3">
            <h2 className="text-sm font-medium text-gray-900">
              Was passiert als nächstes?
            </h2>
            <ul className="space-y-2 text-sm text-gray-600">
              <li className="flex items-start gap-2">
                <span className="mt-1 flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-medium text-gray-600">
                  1
                </span>
                Unser Team prüft deine Anfrage
              </li>
              <li className="flex items-start gap-2">
                <span className="mt-1 flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-medium text-gray-600">
                  2
                </span>
                Du erhältst eine E-Mail mit dem Ergebnis
              </li>
              <li className="flex items-start gap-2">
                <span className="mt-1 flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-medium text-gray-600">
                  3
                </span>
                Nach der Freischaltung kannst du dich anmelden
              </li>
            </ul>
          </div>

          {/* Email Info */}
          <div className="mb-6 rounded-xl border border-blue-100 bg-blue-50/50 p-4">
            <div className="flex items-start gap-3">
              <svg
                className="mt-0.5 h-5 w-5 flex-shrink-0 text-blue-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                />
              </svg>
              <p className="text-sm text-blue-700">
                Wir haben dir eine Bestätigungsmail geschickt. Bitte prüfe auch
                deinen Spam-Ordner.
              </p>
            </div>
          </div>

          {/* Back to Login */}
          <Link
            href="/"
            className="block w-full rounded-xl bg-gray-900 py-3 text-center text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800 hover:shadow-xl"
          >
            Zur Anmeldung
          </Link>
        </div>

        {/* Footer */}
        <p className="text-center text-sm text-gray-500">
          Bei Fragen kannst du dich jederzeit an{" "}
          <a
            href="mailto:support@moto.nrw"
            className="font-medium text-gray-700 hover:text-gray-900"
          >
            support@moto.nrw
          </a>{" "}
          wenden.
        </p>
      </div>
    </div>
  );
}
