import type { ReactNode } from "react";
import { SectionCard } from "~/components/ui/section-card";

/**
 * Kopfkarte einer Seite, die einen Erklärtext oder Seitenaktionen trägt.
 *
 * Entspricht dem Kopf der Eltern-App (`ParentPageHeader`): blauer Kicker,
 * Titel, Erklärtext und Aktionen in EINER Kartenzeile. Ein Erklärtext steht
 * damit nie frei zwischen Seitenkopf und Inhalt, und Aktionen bekommen keine
 * eigene, sonst leere Zeile. Seiten ohne Erklärtext und ohne Seitenaktionen
 * brauchen diese Karte nicht; dort reicht `PageHeaderWithSearch`.
 *
 * `prominent` hebt den Titel eine Stufe über Abschnittsüberschriften, für
 * Startseiten mit Begrüßung als Titel.
 */
export function PageIntro({
  kicker,
  title,
  description,
  actions,
  prominent = false,
  className,
  children,
}: Readonly<{
  kicker?: string;
  title: string;
  description?: string;
  /** Aktionen in der Titelzeile, rechts. */
  actions?: ReactNode;
  prominent?: boolean;
  className?: string;
  /** Inhalt unterhalb des Kopfs, zum Beispiel Kennzahlen oder ein Hinweis. */
  children?: ReactNode;
}>) {
  return (
    <SectionCard
      kicker={kicker}
      title={title}
      description={description}
      actions={actions}
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
