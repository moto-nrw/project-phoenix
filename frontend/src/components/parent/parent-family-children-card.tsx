"use client";

// Kinder in der Familie (#5918): Eltern geben im Vertrag an, wie viele
// Kinder ihre Familie hat, auch Geschwister ohne Betreuungsvertrag. Der
// Träger bestätigt die Zahl; erst dann zählt sie für die
// Geschwisterermäßigung. Gespeichert wird nur die Zahl.

import { useLocale, useTranslations } from "next-intl";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { FieldLabel } from "~/components/ui/field-label";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { NumberStepper } from "~/components/ui/number-stepper";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { parentsConcept } from "~/lib/concepts/parents";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchFamilyChildren,
  type ParentFamilyChildren,
  stateFamilyChildren,
} from "~/lib/parent-family-children-api";

const logger = createLogger({ component: "ParentFamilyChildrenCard" });

const MAX_CHILDREN = 20;

/** The parents' statement waits for the Träger when it is newer than the confirmation. */
function waitsForTraeger(family: ParentFamilyChildren): boolean {
  return (
    family.statedChildren !== null &&
    family.statedChildren !== family.confirmedChildren &&
    family.statedAt !== null &&
    (family.confirmedAt === null ||
      Date.parse(family.statedAt) > Date.parse(family.confirmedAt))
  );
}

export function ParentFamilyChildrenCard({
  organizationId,
  contractId,
}: Readonly<{ organizationId: string; contractId: string }>) {
  const t = useTranslations("parentFamilyChildren");
  const locale = useLocale();
  const [family, setFamily] = useState<ParentFamilyChildren | null>(null);
  const [children, setChildren] = useState(1);
  const [saving, setSaving] = useState(false);
  const loadError = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadError;
  const formRef = useRef<HTMLFormElement>(null);
  const saveError = useApiFormError(formRef);
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      const next = await fetchFamilyChildren(organizationId, contractId);
      setFamily(next);
      setChildren(next.statedChildren ?? next.confirmedChildren ?? 1);
      clearLoadError();
    } catch (err) {
      logger.error("parent_family_children_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: t("errorObject"),
        retry: () => void load(),
      });
    }
  }, [organizationId, contractId, clearLoadError, showLoadError, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const submit = async () => {
    saveError.clear();
    setSaving(true);
    try {
      setFamily(
        await stateFamilyChildren(organizationId, contractId, children),
      );
      success(t("saved"));
    } catch (err) {
      logger.error("parent_family_children_state_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void saveError.show(err, { object: t("errorObject") });
    } finally {
      setSaving(false);
    }
  };

  const date = (value: string | null) =>
    value ? formatDate(value, false, locale) : "";

  return (
    <SectionCard
      concept={parentsConcept}
      title={t("title")}
      description={t("description")}
      loading={!family && !loadError.error}
    >
      {loadError.error ? <LoadErrorAlert error={loadError.error} /> : null}
      {family ? (
        <div className="space-y-4">
          {family.confirmedChildren !== null || waitsForTraeger(family) ? (
            <InsetPanel className="space-y-2">
              {family.confirmedChildren !== null ? (
                <p className="flex flex-wrap items-center gap-2 text-sm text-gray-900">
                  <StatusBadge tone="green" label={t("confirmedBadge")} />
                  {family.validUntil
                    ? t("confirmedUntil", {
                        count: family.confirmedChildren,
                        from: date(family.validFrom),
                        until: date(family.validUntil),
                      })
                    : t("confirmedFrom", {
                        count: family.confirmedChildren,
                        from: date(family.validFrom),
                      })}
                </p>
              ) : null}
              {waitsForTraeger(family) ? (
                <p className="flex flex-wrap items-center gap-2 text-sm text-gray-900">
                  <StatusBadge tone="orange" label={t("pendingBadge")} />
                  {t("pending", {
                    count: family.statedChildren ?? 0,
                    date: date(family.statedAt),
                  })}
                </p>
              ) : null}
            </InsetPanel>
          ) : null}
          {family.canState ? (
            <form
              ref={formRef}
              noValidate
              className="space-y-3"
              onSubmit={(event) => {
                event.preventDefault();
                void submit();
              }}
            >
              <FormErrorAlert message={saveError.error} />
              <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
                <div className="min-w-0">
                  <FieldLabel htmlFor="parent-family-children" required>
                    {t("fieldLabel")}
                  </FieldLabel>
                  <NumberStepper
                    id="parent-family-children"
                    name="children"
                    value={String(children)}
                    onChange={(event) => {
                      const next = Number(event.target.value);
                      if (
                        Number.isInteger(next) &&
                        next >= 1 &&
                        next <= MAX_CHILDREN
                      )
                        setChildren(next);
                    }}
                    onStep={(delta) =>
                      setChildren((current) =>
                        Math.min(MAX_CHILDREN, Math.max(1, current + delta)),
                      )
                    }
                    canDecrement={children > 1}
                    canIncrement={children < MAX_CHILDREN}
                    decrementLabel={t("less")}
                    incrementLabel={t("more")}
                    error={saveError.fieldError("children")}
                  />
                </div>
                <Button
                  type="submit"
                  variant="primary"
                  size="md"
                  isLoading={saving}
                  loadingText={t("sending")}
                >
                  {t("send")}
                </Button>
              </div>
            </form>
          ) : (
            <p className="text-sm text-gray-600">{t("noRight")}</p>
          )}
          <p className="text-sm text-gray-600">{t("onlyNumber")}</p>
        </div>
      ) : null}
    </SectionCard>
  );
}
