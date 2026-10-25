"use client";

// Öffentliche Ferienanmeldung (#4661): der eine Link eines Ferienzeitraums
// für alle Schulen des Trägers. Eltern brauchen kein Konto.

import { useLocale, useTranslations } from "next-intl";
import { use, useCallback, useEffect, useState } from "react";
import { HolidayRegistrationForm } from "~/components/holiday-registration/holiday-registration-form";
import {
  PublicEnrollmentContentSkeleton,
  PublicEnrollmentPageShell,
} from "~/components/enrollment/public-enrollment-shell";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import {
  fetchHolidayRegistrationOffer,
  type HolidayRegistrationOffer,
} from "~/lib/holiday-registration-api";
import { useAnalyticsSchoolPolicy } from "~/lib/hooks/use-analytics-school-policy";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "HolidayRegistrationPage" });

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

export default function HolidayRegistrationPage({ params }: PageProps) {
  // The link names a Träger, no school: no school policy permits collection.
  useAnalyticsSchoolPolicy("", false);
  const { token } = use(params);
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  const [offer, setOffer] = useState<HolidayRegistrationOffer | null>(null);
  const loadError = useApiLoadError();
  const { show, clear } = loadError;

  const load = useCallback(async () => {
    try {
      setOffer(await fetchHolidayRegistrationOffer(token));
      clear();
    } catch (err) {
      logger.error("holiday_registration_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, { object: t("errorObject"), retry: () => void load() });
    }
  }, [clear, show, t, token]);

  useEffect(() => {
    void load();
  }, [load]);

  const dates = offer
    ? {
        opens: formatDate(offer.registrationOpensOn, false, locale),
        closes: formatDate(offer.registrationClosesOn, false, locale),
      }
    : null;

  return (
    <PublicEnrollmentPageShell>
      <div className="mx-auto w-full max-w-3xl space-y-6">
        {loadError.error ? <LoadErrorAlert error={loadError.error} /> : null}
        {!offer && !loadError.error ? (
          <div role="status" aria-label={t("loading")}>
            <PublicEnrollmentContentSkeleton sections={2} />
          </div>
        ) : null}
        {offer && dates ? (
          <>
            <SectionCard
              kicker={`${t("eyebrow")} · ${offer.traegerName}`}
              title={offer.name}
            >
              <div className="space-y-2 text-sm text-gray-700">
                <p>{t("window", dates)}</p>
                {offer.binding ? <p>{t("binding")}</p> : null}
                {offer.notes ? (
                  <div>
                    <p className="font-semibold text-gray-900">
                      {t("notesTitle")}
                    </p>
                    <p className="whitespace-pre-line">{offer.notes}</p>
                  </div>
                ) : null}
              </div>
            </SectionCard>
            {offer.open ? (
              <HolidayRegistrationForm token={token} offer={offer} />
            ) : (
              <SectionCard title={t("closedTitle")}>
                <p className="text-sm text-gray-700">
                  {t("closedText", dates)}
                </p>
              </SectionCard>
            )}
          </>
        ) : null}
      </div>
    </PublicEnrollmentPageShell>
  );
}
