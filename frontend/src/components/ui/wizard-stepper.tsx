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
const INACTIVE_TEXT = "#9CA3AF";

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
        <span style={{ color: INACTIVE_TEXT }}>{steps[clampedCurrent]}</span>
      </div>
    </div>
  );
}
