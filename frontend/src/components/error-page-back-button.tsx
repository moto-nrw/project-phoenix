"use client";

import { ArrowLeft } from "lucide-react";
import { Button } from "~/components/ui/button";

/** „Zurück"-Aktion der Fehlerseite. Browser-History statt fester Route, damit
 * die Person dort landet, wo sie herkam — auf jedem Portal. */
export function ErrorPageBackButton({ label }: Readonly<{ label: string }>) {
  return (
    <Button
      type="button"
      variant="outline"
      size="md"
      className="gap-2"
      onClick={() => window.history.back()}
    >
      <ArrowLeft className="size-4" aria-hidden="true" />
      {label}
    </Button>
  );
}
