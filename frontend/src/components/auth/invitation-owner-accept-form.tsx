"use client";

import { useState } from "react";
import { Button, ButtonLink } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { acceptInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { listAllTenants } from "~/lib/tenant-api";
import type { TenantSummary } from "~/lib/tenant-api";
import { schoolPortalLoginUrl } from "~/lib/school-url";
import { parentsPortalLoginUrl } from "~/lib/parent-url";
import { clientEnv } from "~/env.client";

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
  const [error, setError] = useState<string | null>(null);
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

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setPending(true);
    setError(null);
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
      const code = (err as ApiError).code;
      if (code === "INVITATION_ACCOUNT_LOGIN_REQUIRED") {
        setError(
          "Bitte melden Sie sich zuerst mit der eingeladenen E-Mail-Adresse an.",
        );
      } else if (code === "INVITATION_ACCOUNT_MISMATCH") {
        setError(
          "Sie sind mit einem anderen Konto angemeldet. Bitte wechseln Sie das Konto.",
        );
      } else if (code === "ACCOUNT_INACTIVE") {
        setError("Ihr Konto ist gesperrt. Bitte wenden Sie sich an moto.");
      } else if (
        (err as ApiError).status === 410 ||
        (err as ApiError).status === 404
      ) {
        setError(
          "Diese Einladung ist nicht mehr gültig. Bitte fragen Sie die Schule nach einer neuen Einladung.",
        );
      } else {
        setError(
          "Das hat leider nicht geklappt. Bitte versuchen Sie es erneut.",
        );
      }
    } finally {
      setPending(false);
    }
  }

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
    <form onSubmit={submit} className="space-y-5">
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
              <Alert
                type="error"
                message="Die Schulliste ist gerade nicht verfügbar. Bitte versuchen Sie es erneut."
                action={
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => void showOtherAddresses()}
                  >
                    Erneut laden
                  </Button>
                }
              />
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
          label="Vorname"
          value={firstName}
          onChange={(event) => setFirstName(event.target.value)}
          autoComplete="given-name"
          required
          disabled={pending}
        />
        <Input
          id="owner-last-name"
          label="Nachname"
          value={lastName}
          onChange={(event) => setLastName(event.target.value)}
          autoComplete="family-name"
          required
          disabled={pending}
        />
      </div>
      {error ? <Alert type="error" message={error} /> : null}
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
