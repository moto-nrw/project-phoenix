"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { Button, ButtonLink } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import { createLogger } from "~/lib/logger";
import { listAllTenants } from "~/lib/tenant-api";
import type { TenantSummary } from "~/lib/tenant-api";
import { schoolPortalLoginUrl } from "~/lib/school-url";
import { parentsPortalLoginUrl } from "~/lib/parent-url";
import { clientEnv } from "~/env.client";
import { credentialError } from "./credential-error";

const logger = createLogger({ component: "InvitationOwnerAccept" });

export function InvitationOwnerAcceptForm({
  token,
  invitation,
  redirectToPath,
}: {
  readonly token: string;
  readonly invitation: InvitationValidation;
  readonly redirectToPath?: string;
}) {
  const [firstName, setFirstName] = useState(invitation.firstName ?? "");
  const [lastName, setLastName] = useState(invitation.lastName ?? "");
  const [pending, setPending] = useState(false);
  const [accepted, setAccepted] = useState(false);
  const [destination, setDestination] = useState<string | null>(null);
  // Fehler über den gemeinsamen Weg (#2517): Katalogtext je Code. "Erst
  // anmelden" kommt als 401; das ist hier eine Ablehnung, kein
  // Sitzungsende, deshalb credentialError.
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const latestSubmitRef = useRef<() => void>(() => undefined);
  const [tenants, setTenants] = useState<TenantSummary[]>([]);
  const [showAddresses, setShowAddresses] = useState(false);
  const [addressStatus, setAddressStatus] = useState<
    "idle" | "loading" | "error" | "ready"
  >("idle");

  function openInvitationAt(url: URL) {
    url.pathname = "/invite";
    url.search = new URLSearchParams({ token }).toString();
    window.location.assign(url.href);
  }

  async function showOtherAddresses() {
    setShowAddresses(true);
    setAddressStatus("loading");
    const result = await listAllTenants();
    setTenants(result.tenants);
    setAddressStatus(
      result.status === "error" || result.tenants.length === 0
        ? "error"
        : "ready",
    );
  }

  async function accept() {
    setPending(true);
    formErrors.clear();
    try {
      const result = await acceptInvitation(token, {
        existingAccount: true,
        firstName,
        lastName,
        password: "",
        confirmPassword: "",
      });
      if (invitation.targetPortal === "school") {
        setDestination(schoolPortalLoginUrl());
      } else if (result.tenantSubdomain) {
        const port = window.location.port ? `:${window.location.port}` : "";
        setDestination(
          `${window.location.protocol}//${result.tenantSubdomain}.${clientEnv.NEXT_PUBLIC_TENANT_DOMAIN}${port}/`,
        );
      }
      setAccepted(true);
    } catch (err) {
      logger.warn("invitation_owner_accept_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(
        credentialError(err, ["identity.invitation_account_login_required"]),
        { object: "die Einladung", retry: () => latestSubmitRef.current() },
      );
    } finally {
      setPending(false);
    }
  }

  function submit(event: React.FormEvent) {
    event.preventDefault();
    void accept();
  }

  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void accept();
  });

  if (accepted) {
    return (
      <div className="space-y-4">
        <Alert
          type="success"
          title="Einladung angenommen"
          message="Sie haben jetzt Zugang zur neuen Schule. Ihr Passwort bleibt unverändert. Ihre bisherigen Zugänge bleiben bestehen."
        />
        {destination ? (
          <ButtonLink href={destination} size="md">
            Zur neuen Schule
          </ButtonLink>
        ) : null}
      </div>
    );
  }

  return (
    <form ref={formRef} onSubmit={submit} className="space-y-5">
      <FormErrorAlert message={formErrors.error} />
      <p className="text-sm text-gray-700">
        Einladung für <strong>{invitation.email}</strong>.
      </p>
      <ol className="list-decimal space-y-2 pl-5 text-sm text-gray-700">
        <li>Melden Sie sich mit dieser E-Mail-Adresse bei moto an.</li>
        <li>Kehren Sie danach zu dieser Einladung zurück.</li>
        <li>Prüfen Sie Ihren Namen und nehmen Sie die Einladung an.</li>
      </ol>
      <ButtonLink
        href={redirectToPath ?? "/"}
        target="_blank"
        rel="noopener noreferrer"
        variant="outline"
        size="md"
      >
        Anmeldung öffnen (neuer Tab)
      </ButtonLink>
      <div>
        <Button
          type="button"
          variant="ghost"
          size="md"
          onClick={() => void showOtherAddresses()}
          disabled={addressStatus === "loading"}
        >
          Andere moto-Adresse wählen
        </Button>
        {showAddresses ? (
          <div className="mt-3 space-y-3">
            <p className="text-sm text-gray-600">
              Wählen Sie den Bereich, in dem Sie moto bisher nutzen.
            </p>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() =>
                  openInvitationAt(new URL(parentsPortalLoginUrl()))
                }
              >
                Für Eltern
              </Button>
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() =>
                  openInvitationAt(new URL(schoolPortalLoginUrl()))
                }
              >
                Für Lehrkräfte
              </Button>
            </div>
            {addressStatus === "loading" ? (
              <p role="status" className="text-sm text-gray-600">
                Schulen werden geladen…
              </p>
            ) : null}
            {addressStatus === "error" ? (
              // listAllTenants gibt nur "error" zurück, kein Fehlerobjekt:
              // ein fester Satz, Wiederholen über den Knopf darunter.
              <div className="space-y-2">
                <LoadErrorAlert error="Die Liste der Schulen ist gerade nicht erreichbar. Bitte versuchen Sie es erneut." />
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={() => void showOtherAddresses()}
                >
                  Erneut laden
                </Button>
              </div>
            ) : null}
            {addressStatus === "ready" ? (
              <>
                <label
                  id="invitation-school-label"
                  htmlFor="invitation-school"
                  className="block text-sm text-gray-700"
                >
                  Für Betreuungskräfte: bisherige Schule
                </label>
                <CustomSelect
                  id="invitation-school"
                  ariaLabelledBy="invitation-school-label"
                  value=""
                  placeholder="Schule wählen"
                  options={tenants.map((tenant) => ({
                    value: tenant.subdomain,
                    label: tenant.name,
                  }))}
                  onChange={(subdomain) => {
                    if (
                      !tenants.some((tenant) => tenant.subdomain === subdomain)
                    )
                      return;
                    const port = window.location.port
                      ? `:${window.location.port}`
                      : "";
                    openInvitationAt(
                      new URL(
                        `${window.location.protocol}//${subdomain}.${clientEnv.NEXT_PUBLIC_TENANT_DOMAIN}${port}`,
                      ),
                    );
                  }}
                />
              </>
            ) : null}
          </div>
        ) : null}
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <Input
          id="owner-first-name"
          name="first_name"
          label="Vorname"
          error={formErrors.fieldError("first_name")}
          value={firstName}
          onChange={(event) => setFirstName(event.target.value)}
          autoComplete="given-name"
          required
          disabled={pending}
        />
        <Input
          id="owner-last-name"
          name="last_name"
          label="Nachname"
          error={formErrors.fieldError("last_name")}
          value={lastName}
          onChange={(event) => setLastName(event.target.value)}
          autoComplete="family-name"
          required
          disabled={pending}
        />
      </div>
      <Button
        type="submit"
        isLoading={pending}
        loadingText="Wird angenommen…"
        disabled={pending}
      >
        Einladung annehmen
      </Button>
    </form>
  );
}
