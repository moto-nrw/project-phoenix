"use client";

import { useCallback, useLayoutEffect, useRef, useState } from "react";

import { AuthShell, MotoBrand } from "~/components/auth/auth-shell";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { downloadPayrollFile } from "~/lib/payroll-download-api";

const logger = createLogger({ component: "PayrollDownloadContent" });

const UNAVAILABLE_CODE = "workforce.payroll_download_unavailable";

type DownloadState = "idle" | "loading" | "done" | "gone";

export function PayrollDownloadContent({ link }: { readonly link: string }) {
  const [state, setState] = useState<DownloadState>("idle");
  const formRef = useRef<HTMLDivElement>(null);
  const { error, show, clear } = useApiFormError(formRef);
  const retryRef = useRef<() => void>(() => undefined);

  const download = useCallback(async () => {
    setState("loading");
    clear();
    try {
      await downloadPayrollFile(link);
      setState("done");
    } catch (err) {
      const gone =
        err instanceof ApiError && wireErrorCode(err.code) === UNAVAILABLE_CODE;
      const context = {
        error: err instanceof Error ? err.message : String(err),
      };
      // Ein abgelaufener oder verbrauchter Link ist kein Defekt.
      if (gone) logger.warn("payroll_download_unavailable", context);
      else logger.error("payroll_download_failed", context);
      setState(gone ? "gone" : "idle");
      // Der Katalogtext nennt den Grund; nur ein anderer Fehler bietet
      // Wiederholen, denn ein verbrauchter Link öffnet sich nicht wieder.
      void show(err, {
        object: "die Datei",
        ...(gone ? {} : { retry: () => retryRef.current() }),
      });
    }
  }, [clear, link, show]);

  useLayoutEffect(() => {
    retryRef.current = () => void download();
  });

  return (
    <AuthShell
      eyebrow="Lohnbüro"
      eyebrowClassName="text-moto-green"
      title="Monatsdatei herunterladen"
      subtitle="Eine Schule schickt Ihnen die Arbeitszeiten ihres Teams für einen Monat."
      variant="tenant"
      brand={<MotoBrand />}
      formMaxWidth="max-w-[32rem]"
    >
      <div ref={formRef} className="space-y-4">
        <FormErrorAlert message={error} />
        {state === "done" ? (
          <Alert
            type="success"
            message="Die Datei ist heruntergeladen. Der Link funktioniert jetzt nicht mehr."
          />
        ) : state === "gone" ? null : (
          <>
            <p className="text-sm leading-6 text-gray-600">
              Sie können die Datei nur einmal herunterladen. Speichern Sie sie
              danach gut.
            </p>
            <Button
              type="button"
              variant="primary"
              size="md"
              className="w-full"
              isLoading={state === "loading"}
              loadingText="Wird heruntergeladen …"
              onClick={() => void download()}
            >
              Datei herunterladen
            </Button>
          </>
        )}
      </div>
    </AuthShell>
  );
}
