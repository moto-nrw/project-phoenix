"use client";

import { MotoBrand } from "~/components/auth/moto-brand";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";

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
      <div className="relative flex flex-col items-center">
        <MotoBrand />
        <EmptyState
          titleAs="h1"
          className="mt-4"
          title="Bitte kurz warten"
          description="Die Seite lädt gerade nicht. Das liegt nicht an Ihnen. Bitte warten Sie eine Minute. Laden Sie die Seite dann neu."
          action={
            <Button
              type="button"
              variant="primary"
              size="base"
              onClick={() => globalThis.location.reload()}
            >
              Neu laden
            </Button>
          }
        />
      </div>
    </main>
  );
}
