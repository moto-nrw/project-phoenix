import { Check } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";

export default async function EnrollSubmittedPage() {
  const t = await getTranslations("enrollmentPublic");
  return (
    <PublicEnrollmentPageShell>
      <section className="moto-content-surface mx-auto max-w-4xl rounded-2xl border p-6 text-center shadow-sm sm:p-10">
        <div className="bg-moto-green/15 text-moto-green-strong mx-auto flex h-14 w-14 items-center justify-center rounded-xl">
          <Check className="h-7 w-7" />
        </div>
        <p className="text-moto-blue mt-6 text-sm font-semibold tracking-wide uppercase">
          {t("submittedEyebrow")}
        </p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
          {t("submittedTitle")}
        </h1>
        <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-gray-600">
          {t("submittedText")}
        </p>
      </section>
    </PublicEnrollmentPageShell>
  );
}
