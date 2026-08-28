import { MapPinOff } from "lucide-react";
import { useTranslations } from "next-intl";
import { ErrorPage, ErrorPageIconVisual } from "~/components/error-page";
import { ErrorPageBackButton } from "~/components/error-page-back-button";

/**
 * Ganzseitiger Hinweis, wenn die Adresse zu keiner Schule gehört (#2624).
 * Wird vom Tenant-Layout direkt gerendert — ein `notFound()` im Layout würde
 * in der Root-Boundary landen und die generische „Seite nicht
 * gefunden"-Seite zeigen. Bewusst ohne „Zur Startseite": auf dieser Adresse
 * gibt es keine Startseite. Texte kommen wie alle Fehlerseiten-Texte aus dem
 * Nachrichtenkatalog (auf Staff-Oberflächen immer Deutsch).
 */
export function TenantNotFoundScreen() {
  const t = useTranslations("tenantNotFoundPage");

  return (
    <ErrorPage
      visual={<ErrorPageIconVisual icon={MapPinOff} />}
      title={t("title")}
      description={t("description")}
      actions={<ErrorPageBackButton label={t("back")} />}
    />
  );
}
