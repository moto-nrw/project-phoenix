"use client";

// „Einstellungen → Anmeldung“ im Eltern-Portal (#7051): die freiwillige
// Anmeldung mit Code per E-Mail und, solange sie eingeschaltet ist, die
// gemerkten Geräte. Beide Karten sind die der anderen Portale, in der
// Sprache der Eltern.

import { useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import {
  EnvelopeSimpleIcon,
  DevicesIcon,
  PasswordIcon,
} from "@phosphor-icons/react";

import {
  EmailCodeFactorSection,
  type EmailCodeFactorCopy,
} from "~/components/settings/email-code-factor-section";
import {
  TrustedDevicesSection,
  type TrustedDevicesCopy,
} from "~/components/settings/trusted-devices-section";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { IconTile } from "~/components/ui/icon-tile";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionLabel } from "~/components/ui/section-label";
import { emailCodeFactorApi } from "~/lib/email-code-factor-api";
import { signInConcept } from "~/lib/concepts/signIn";

const parentFactorApi = emailCodeFactorApi("/api/parent/me/mfa");

type Translate = ReturnType<typeof useTranslations<"parentSignIn">>;

function factorCopy(t: Translate): EmailCodeFactorCopy {
  return {
    title: t("factor.title"),
    description: t("factor.description"),
    enabled: t("factor.enabled"),
    disabled: t("factor.disabled"),
    switchOn: t("factor.switchOn"),
    switchOnTooltip: t("factor.switchOnTooltip"),
    switchOff: t("factor.switchOff"),
    switchOffTooltip: t("factor.switchOffTooltip"),
    codeGoesTo: t("factor.codeGoesTo"),
    sentTo: (email) => t("factor.sentTo", { email }),
    codeLabel: t("factor.codeLabel"),
    confirm: t("factor.confirm"),
    confirmTooltip: t("factor.confirmTooltip"),
    resend: t("factor.resend"),
    resendTooltip: t("factor.resendTooltip"),
    cancel: t("factor.cancel"),
    disableIntro: t("factor.disableIntro"),
    disableTooltip: t("factor.disableTooltip"),
    currentPassword: t("factor.currentPassword"),
    enabledToast: t("factor.enabledToast"),
    disabledToast: t("factor.disabledToast"),
    renewFailed: t("factor.renewFailed"),
    reauthenticate: t("factor.reauthenticate"),
    reauthenticateTooltip: t("factor.reauthenticateTooltip"),
    unavailable: t("factor.unavailable"),
    loadObject: t("factor.loadObject"),
    startObject: t("factor.startObject"),
    confirmObject: t("factor.confirmObject"),
    disableObject: t("factor.disableObject"),
  };
}

function devicesCopy(t: Translate, locale: string): TrustedDevicesCopy {
  return {
    title: t("devices.title"),
    description: t("devices.description"),
    emptyTitle: t("devices.emptyTitle"),
    emptyDescription: t("devices.emptyDescription"),
    added: t("devices.added"),
    validUntil: t("devices.validUntil"),
    lastUsed: t("devices.lastUsed"),
    actionsFor: (device) => t("devices.actionsFor", { device }),
    remove: t("devices.remove"),
    removeDescription: t("devices.removeDescription"),
    confirmTitle: t("devices.confirmTitle"),
    confirmText: (device) =>
      t.rich("devices.confirmText", { device: "", b: () => device }),
    confirmFirstStep: t("devices.confirmFirstStep"),
    confirmFinal: t("devices.confirmFinal"),
    removing: t("devices.removing"),
    removedToast: t("devices.removedToast"),
    loadObject: t("devices.loadObject"),
    reloadObject: t("devices.reloadObject"),
    removeObject: t("devices.removeObject"),
    dateLocale: locale,
  };
}

/** The three steps of a sign-in with a code, while it is still off. */
function HowItWorks({ t }: { readonly t: Translate }) {
  const steps = [
    {
      icon: <PasswordIcon aria-hidden="true" />,
      text: t("factor.stepPassword"),
    },
    {
      icon: <EnvelopeSimpleIcon aria-hidden="true" />,
      text: t("factor.stepCode"),
    },
    {
      icon: <DevicesIcon aria-hidden="true" />,
      text: t("factor.stepRemember"),
    },
  ];
  return (
    <InsetPanel padding="md" className="space-y-3">
      <SectionLabel as="h3">{t("factor.howTitle")}</SectionLabel>
      <ol className="space-y-3">
        {steps.map((step) => (
          <li key={step.text} className="flex items-start gap-3">
            <IconTile size="xs" tone="neutral" icon={step.icon} />
            <span className="pt-1.5 text-sm leading-relaxed text-gray-700">
              {step.text}
            </span>
          </li>
        ))}
      </ol>
    </InsetPanel>
  );
}

export function ParentSignInSection() {
  const t = useTranslations("parentSignIn");
  const locale = useLocale();
  const [enabled, setEnabled] = useState(false);
  const factor = useMemo(() => factorCopy(t), [t]);
  const devices = useMemo(() => devicesCopy(t, locale), [t, locale]);

  return (
    <>
      <EmailCodeFactorSection
        api={parentFactorApi}
        copy={factor}
        leading={<ConceptIconTile concept={signInConcept} size="section" />}
        onEnabledChange={setEnabled}
        renderIdle={(on, available) =>
          available || on ? (
            <>
              {on ? null : <HowItWorks t={t} />}
              <p className="text-sm leading-relaxed text-gray-600">
                {t("factor.accountNote")} {t("factor.recoveryNote")}
              </p>
            </>
          ) : null
        }
      />
      {enabled && (
        <TrustedDevicesSection scope="parent" copy={devices} headingLevel={2} />
      )}
    </>
  );
}
