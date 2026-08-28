import { getTranslations } from "next-intl/server";
import { ErrorPage, ErrorPage404Visual } from "~/components/error-page";
import { ErrorPageBackButton } from "~/components/error-page-back-button";
import { ButtonLink } from "~/components/ui/button";

/**
 * App-weite 404 für alle Portale: Next.js rendert diese Datei für jede URL,
 * die keine Route trifft (Root-not-found-Konvention). „Zur Startseite" führt
 * auf `/` — der Proxy löst das je Host zum richtigen Portal-Einstieg auf.
 *
 * Übersetzt über next-intl: auf Parents-Oberflächen setzt der Proxy den
 * Localize-Header und die Portalsprache greift, überall sonst bleibt Deutsch.
 */
export default async function NotFound() {
  const t = await getTranslations("notFoundPage");

  return (
    <ErrorPage
      visual={<ErrorPage404Visual />}
      title={t("title")}
      description={t("description")}
      actions={
        <>
          <ErrorPageBackButton label={t("back")} />
          <ButtonLink href="/" variant="primary" size="md">
            {t("home")}
          </ButtonLink>
        </>
      }
    />
  );
}
