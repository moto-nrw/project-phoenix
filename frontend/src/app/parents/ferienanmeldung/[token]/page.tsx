"use client";

// Öffentliche Ferienanmeldung (#4661): der eine Link eines Ferienzeitraums
// für alle Schulen des Trägers. Eltern brauchen kein Konto. Mit Elternkonto
// sind die bekannten Kinder und die eigenen Angaben vorbefüllt (#4662).

import { useSession } from "next-auth/react";
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
  fetchHolidayRegistrationFamily,
  fetchHolidayRegistrationOffer,
  type HolidayKnownFamily,
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
  const { data: session, status } = useSession();
  const parentSession =
    status === "authenticated" && session?.user?.scope === "parent";
  const [offer, setOffer] = useState<HolidayRegistrationOffer | null>(null);
  const [family, setFamily] = useState<HolidayKnownFamily | null>(null);
  const loadError = useApiLoadError();
  const { show, clear } = loadError;

  const load = useCallback(async () => {
    try {
      const [loadedOffer, loadedFamily] = await Promise.all([
        fetchHolidayRegistrationOffer(token),
        parentSession
          ? fetchHolidayRegistrationFamily(token).catch((err: unknown) => {
              // Deliberately silent: the prefill is a convenience. Without
              // it the parent fills in the form like everyone else.
              logger.warn("holiday_registration_family_load_failed", {
                error: err instanceof Error ? err.message : String(err),
              });
              return null;
            })
          : Promise.resolve(null),
      ]);
      setFamily(loadedFamily);
      setOffer(loadedOffer);
      clear();
    } catch (err) {
      logger.error("holiday_registration_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await show(err, { object: t("errorObject"), retry: () => void load() });
    }
  }, [clear, parentSession, show, t, token]);

  // The Träger changed the form while the parent filled it in (#5198): read
  // the current questions again. A failure leaves the old offer; the next
  // submit names the change once more.
  const reloadOffer = useCallback(async () => {
    try {
      setOffer(await fetchHolidayRegistrationOffer(token));
    } catch (err) {
      logger.warn("holiday_registration_offer_reload_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }, [token]);

  useEffect(() => {
    // Wait for the session, so a parent's form starts prefilled.
    if (status === "loading") return;
    void load();
  }, [load, status]);

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
                {offer.open ? <p>{t("requiredHint")}</p> : null}
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
              <HolidayRegistrationForm
                token={token}
                offer={offer}
                family={family}
                onFormChanged={reloadOffer}
              />
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
