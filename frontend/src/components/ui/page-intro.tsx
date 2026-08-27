import type { ReactNode } from "react";
import { SectionCard } from "~/components/ui/section-card";

/**
 * Kopfkarte einer Seite, die einen Erklärtext oder Seitenaktionen trägt.
 *
 * Titel, Erklärtext und Aktionen in EINER Kartenzeile. Ein Erklärtext steht
 * damit nie frei zwischen Seitenkopf und Inhalt, und Aktionen bekommen keine
 * eigene, sonst leere Zeile. Seiten ohne Erklärtext und ohne Seitenaktionen
 * brauchen diese Karte nicht; dort reicht `PageHeaderWithSearch`.
 *
 * `prominent` hebt den Titel eine Stufe über Abschnittsüberschriften, für
 * Startseiten mit Begrüßung als Titel.
 */
export function PageIntro({
  title,
  description,
  actions,
  leading,
  prominent = false,
  className,
  children,
}: Readonly<{
  title: string;
  description?: ReactNode;
  /** Aktionen in der Titelzeile, rechts. */
  actions?: ReactNode;
  /** Optionales Element links vom Titel, z. B. Avatar oder Konzept-Icon. */
  leading?: ReactNode;
  prominent?: boolean;
  className?: string;
  /** Inhalt unterhalb des Kopfs, zum Beispiel Kennzahlen oder ein Hinweis. */
  children?: ReactNode;
}>) {
  return (
    <SectionCard
      title={title}
      description={description}
      actions={actions}
      leading={leading}
      overflow="visible"
      bodyClassName="mt-4 space-y-4"
      headingLevel={1}
      titleClassName={
        prominent
          ? "text-2xl leading-tight tracking-tight sm:text-[28px]"
          : "text-xl leading-tight tracking-tight sm:text-2xl"
      }
      className={className}
    >
      {children}
    </SectionCard>
  );
}
