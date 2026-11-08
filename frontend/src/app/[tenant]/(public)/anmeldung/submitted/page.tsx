import { Check } from "lucide-react";
import { getTranslations } from "next-intl/server";
import { PublicEnrollmentPageShell } from "~/components/enrollment/public-enrollment-shell";
import { SectionCard } from "~/components/ui/section-card";

export default async function EnrollSubmittedPage() {
  const t = await getTranslations("enrollmentPublic");
  return (
    <PublicEnrollmentPageShell>
      <SectionCard className="mx-auto max-w-4xl text-center">
        <div className="bg-moto-green/15 text-moto-green-strong mx-auto flex h-14 w-14 items-center justify-center rounded-xl">
          <Check className="h-7 w-7" />
        </div>
        <h1 className="mt-6 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
          {t("submittedTitle")}
        </h1>
        <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-gray-600">
          {t("submittedText")}
        </p>
      </SectionCard>
    </PublicEnrollmentPageShell>
  );
}
