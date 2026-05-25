import { Check } from "lucide-react";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";

export default function EnrollSubmittedPage() {
  return (
    <PublicEnrollmentPageShell>
      <section className="moto-content-surface mx-auto max-w-4xl rounded-3xl border p-6 text-center shadow-sm sm:p-10">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-[#83CD2D]/15 text-[#5A8E1F]">
          <Check className="h-7 w-7" />
        </div>
        <p className="mt-6 text-sm font-semibold tracking-wide text-[#5080D8] uppercase">
          Anmeldung eingegangen
        </p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
          Danke. Ihre Anmeldung wurde übermittelt.
        </h1>
        <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-gray-600">
          Eine Bestätigungs-E-Mail mit dem Status-Link ist unterwegs. Über den
          Link können Sie jederzeit den aktuellen Stand einsehen.
        </p>
      </section>
    </PublicEnrollmentPageShell>
  );
}
