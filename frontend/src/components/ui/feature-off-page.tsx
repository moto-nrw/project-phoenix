"use client";

import { PlugsIcon } from "@phosphor-icons/react";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage } from "~/components/ui/tenant-page";

interface FeatureOffPageProps {
  /** Seitentitel — derselbe wie im eingeschalteten Zustand. */
  readonly title: string;
  /** Was hier stünde, wenn die Funktion an wäre. Ein Satz. */
  readonly description: string;
  /**
   * Wer sie einschalten kann. Für Funktionen, die nur moto schaltet
   * (NFC, Anwesenheitsmodus), ist das ausdrücklich NICHT die Leitung.
   */
  readonly whoCanEnable?: string;
  readonly embedded?: boolean;
}

/**
 * Eine für diese Schule nicht eingeschaltete Funktion ist ein ZUSTAND, kein
 * Fehler und erst recht kein „nicht gefunden".
 *
 * Vorher riefen die Wächter `notFound()`, und Next.js zeigte daraufhin die
 * 404-Seite des Mandanten: „Schule nicht gefunden — Die angegebene Schule
 * existiert nicht oder ist nicht aktiv. Bitte überprüfen Sie die Subdomain."
 * Für jemanden, der angemeldet in seiner eigenen Schule steht, ist jedes Wort
 * davon falsch, und der Rat führt in die Irre.
 *
 * `.claude/rules/verstaendlichkeit.md`: Wo eine Person nichts ändern kann,
 * sagt der Bildschirm, wer als Nächstes handelt.
 */
export function FeatureOffPage({
  title,
  description,
  whoCanEnable = "Wenden Sie sich an moto, wenn Sie diese Funktion nutzen möchten.",
  embedded = false,
}: FeatureOffPageProps) {
  const icon = <PlugsIcon className="h-12 w-12" aria-hidden="true" />;
  const body = `${description} ${whoCanEnable}`;

  if (embedded) {
    return (
      <EmptyState
        icon={icon}
        title="Diese Funktion ist nicht eingeschaltet"
        description={body}
      />
    );
  }

  return (
    <TenantPage
      title={title}
      empty={{
        title: "Diese Funktion ist nicht eingeschaltet",
        description: body,
        icon,
      }}
    />
  );
}
