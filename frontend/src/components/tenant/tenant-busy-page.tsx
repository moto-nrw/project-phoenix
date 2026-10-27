"use client";

import { MotoBrand } from "~/components/auth/moto-brand";
import { Button } from "~/components/ui/button";

/**
 * Shown by the tenant layout when the backend rate limit rejected the school
 * lookup (HTTP 429, #5090). The layout renders it instead of throwing, so no
 * error boundary and no Sentry event is involved. A full page load is the
 * retry: it runs the server layout and its lookup again, which an error
 * boundary's retry or a client refresh of the failed segment would not.
 */
export function TenantBusyPage() {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen flex min-h-dvh flex-col items-center justify-center px-4 py-16">
      <div className="relative flex flex-col items-center text-center">
        <MotoBrand />
        <h1 className="mt-8 text-3xl font-semibold text-gray-950 sm:text-4xl">
          Bitte kurz warten
        </h1>
        <p className="mt-3 max-w-md text-sm leading-6 text-gray-600 sm:text-base">
          Die Seite lädt gerade nicht. Das liegt nicht an Ihnen.
        </p>
        <p className="mt-1 max-w-md text-sm leading-6 text-gray-600 sm:text-base">
          Bitte warten Sie eine Minute. Laden Sie die Seite dann neu.
        </p>
        <Button
          type="button"
          variant="primary"
          size="base"
          className="mt-9"
          onClick={() => globalThis.location.reload()}
        >
          Neu laden
        </Button>
      </div>
    </main>
  );
}
