"use client";

import { House, Plus, Share, type LucideIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { cn } from "~/lib/utils";

const INSTALL_STEPS: readonly {
  readonly step: 1 | 2 | 3;
  readonly icon: LucideIcon;
}[] = [
  { step: 1, icon: Share },
  { step: 2, icon: Plus },
  { step: 3, icon: House },
];

export function PushInstallSteps({
  className,
  compact = false,
}: Readonly<{ className?: string; compact?: boolean }>) {
  const t = useTranslations("pushNotifications");

  return (
    <div className={cn("space-y-4", className)}>
      <p
        className={cn(
          "max-w-2xl text-pretty text-gray-700",
          compact ? "text-sm leading-6" : "text-base leading-7",
        )}
      >
        {t("installIntro")}
      </p>
      <ol className="space-y-3" aria-label={t("installStepsLabel")}>
        {INSTALL_STEPS.map(({ step, icon: StepIcon }) => (
          <li key={step} className="flex items-start gap-3">
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-100 text-sm font-semibold text-gray-800">
              {step}
            </span>
            <div
              className={cn(
                "flex min-w-0 flex-1 items-start gap-2 rounded-xl bg-gray-50 px-3",
                compact ? "py-2" : "py-2.5",
              )}
            >
              <StepIcon
                className="mt-0.5 h-5 w-5 shrink-0 text-gray-600"
                strokeWidth={2}
                aria-hidden="true"
              />
              <span
                className={cn(
                  "text-pretty text-gray-700",
                  compact ? "text-sm leading-5" : "text-base leading-6",
                )}
              >
                {t(`installStep${step}`)}
              </span>
            </div>
          </li>
        ))}
      </ol>
      <p
        className={cn(
          "max-w-2xl text-pretty text-gray-700",
          compact ? "text-sm leading-6" : "text-base leading-7",
        )}
      >
        {t("installOutro")}
      </p>
    </div>
  );
}
