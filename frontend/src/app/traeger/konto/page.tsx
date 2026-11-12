"use client";

// Konto-Seite des Träger-Portals (#4693): die Anmelde-Adresse zum Lesen und
// der eigene Name in diesem Träger-Büro. Den Namen zeigen das
// Exportprotokoll und der Verlauf jeder Rechnung statt einer E-Mail-Adresse.
// Passwort und Code gehören zum moto-Konto und haben hier keine Karte.

import { useCallback, useEffect, useRef, useState } from "react";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  changeTraegerProfileName,
  fetchTraegerProfile,
  type TraegerProfile,
} from "~/lib/traeger-api";
import {
  OWN_NAME_REQUIRED,
  TRAEGER_NAME_HINT,
  TRAEGER_NAME_MAX_LENGTH,
} from "~/lib/traeger-person-name";

const logger = createLogger({ component: "TraegerAccountPage" });

/** The own name: shown in a field, saved with „Speichern“ (no autosave). */
function NameCard({
  name,
  onSaved,
}: Readonly<{
  name: string;
  onSaved: (profile: TraegerProfile) => void;
}>) {
  const [draft, setDraft] = useState(name);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

  const changed = draft !== name;

  const submit = async () => {
    formErrors.clear();
    const trimmed = draft.trim();
    if (!trimmed) {
      formErrors.invalid(OWN_NAME_REQUIRED, { name: OWN_NAME_REQUIRED });
      return;
    }
    setSaving(true);
    let saved: TraegerProfile;
    try {
      saved = await changeTraegerProfileName(trimmed);
    } catch (err) {
      logger.error("traeger_profile_name_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "Ihr Name" });
      setSaving(false);
      return;
    }
    setSaving(false);
    // The field shows what the backend keeps (trimmed).
    setDraft(saved.name);
    onSaved(saved);
    success("Ihr Name ist gespeichert.");
  };

  return (
    <SectionCard title="Name" description={TRAEGER_NAME_HINT}>
      <form
        ref={formRef}
        noValidate
        className="max-w-xl space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        <Input
          controlSize="compact"
          id="traeger-profile-name"
          label="Ihr Name"
          name="name"
          autoComplete="name"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          error={formErrors.fieldError("name")}
          maxLength={TRAEGER_NAME_MAX_LENGTH}
          disabled={saving}
          required
        />
        {changed ? (
          <EditActions
            onCancel={() => {
              setDraft(name);
              formErrors.clear();
            }}
            saving={saving}
            saveType="submit"
          />
        ) : null}
      </form>
    </SectionCard>
  );
}

export default function TraegerAccountPage() {
  const [profile, setProfile] = useState<TraegerProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const profileLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = profileLoad;

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setProfile(await fetchTraegerProfile());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_profile_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "Ihr Konto",
        retry: () => void load(),
      });
    } finally {
      setLoading(false);
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <TenantPage
      title="Konto"
      loading={loading && !profile}
      error={profile ? null : profileLoad.error}
    >
      {profile ? (
        <>
          <SectionCard
            title="Anmeldung"
            description="Mit dieser E-Mail-Adresse melden Sie sich an."
          >
            <DataGrid>
              <DataField label="E-Mail" fullWidth>
                <span className="wrap-anywhere">{profile.email}</span>
              </DataField>
            </DataGrid>
          </SectionCard>
          <NameCard name={profile.name} onSaved={setProfile} />
        </>
      ) : null}
    </TenantPage>
  );
}
