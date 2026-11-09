"use client";

import { LOCATION_COLORS } from "~/lib/location-helper";

interface WizardStepperProps {
  readonly steps: readonly string[];
  readonly current: number;
  readonly className?: string;
}

const ACTIVE = LOCATION_COLORS.GROUP_ROOM;
const INACTIVE = "#E5E7EB";
const ACTIVE_TEXT = "#1F2937";

export function WizardStepper({
  steps,
  current,
  className = "",
}: Readonly<WizardStepperProps>) {
  const total = steps.length;
  const clampedCurrent = Math.max(0, Math.min(current, total - 1));

  return (
    <div
      className={`flex flex-col gap-3 ${className}`}
      aria-label={`Schritt ${clampedCurrent + 1} von ${total}`}
    >
      <div className="flex items-center gap-2">
        {steps.map((label, index) => {
          const isCompleted = index < clampedCurrent;
          const isCurrent = index === clampedCurrent;
          const isActive = isCompleted || isCurrent;
          return (
            <div
              key={label}
              className="flex flex-1 items-center"
              aria-current={isCurrent ? "step" : undefined}
            >
              <div
                className="h-2 flex-1 rounded-full transition-colors duration-300"
                style={{ backgroundColor: isActive ? ACTIVE : INACTIVE }}
              />
            </div>
          );
        })}
      </div>
      <div className="flex items-center justify-between text-xs">
        <span className="font-medium" style={{ color: ACTIVE_TEXT }}>
          Schritt {clampedCurrent + 1} von {total}
        </span>
        {/* The name of the current step is read, not decoration: it keeps
            the contrast of body text (WCAG 1.4.3). */}
        <span className="text-gray-600">{steps[clampedCurrent]}</span>
      </div>
    </div>
  );
}
