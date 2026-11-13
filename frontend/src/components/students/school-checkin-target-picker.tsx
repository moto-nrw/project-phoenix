"use client";

import { DoorOpen } from "lucide-react";
import { useId } from "react";

import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldLabel } from "~/components/ui/field-label";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import {
  checkinTargetLabel,
  type CheckinTargetState,
} from "~/lib/hooks/use-checkin-target";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { cn } from "~/lib/utils";

/** The value of „Anwesend (ohne Raum)“ in the select. */
const WITHOUT_ROOM = "";
/** From this many targets on, the menu gets a search field. */
const SEARCH_FROM = 8;

interface SchoolCheckinTargetPickerProps {
  readonly state: CheckinTargetState;
  /** Locked while a bulk request runs: the run uses the target it started with. */
  readonly disabled?: boolean;
  /** `bar` for the desktop bar, `sheet` for the phone's sticky bar. */
  readonly layout?: "bar" | "sheet";
  readonly className?: string;
}

/**
 * „Anmelden in“: where a check-in in the app puts the children (#6953).
 * Default is „Anwesend (ohne Raum)“, the check-in as before; a room or a
 * running activity puts the children there in the same step, as a scan at
 * the room's terminal would. The choice stays until it is changed.
 */
export function SchoolCheckinTargetPicker({
  state,
  disabled = false,
  layout = "bar",
  className,
}: SchoolCheckinTargetPickerProps) {
  const labelId = useId();
  const hintId = useId();
  const loadError = useSwrLoadError(
    state.loadError,
    "die Räume für die Anmeldung",
    () => void state.refresh(),
    "Bis dahin melden Sie Kinder ohne Raum an.",
  );
  if (!state.available) return null;

  const { target } = state;
  const options = [
    { value: WITHOUT_ROOM, label: "Anwesend (ohne Raum)" },
    ...state.options.map((option) => ({
      value: option.value,
      label: checkinTargetLabel(option),
    })),
  ];
  const sheet = layout === "sheet";

  return (
    <div className={cn("min-w-0", className)}>
      <div
        className={cn(
          "flex min-w-0 items-center gap-x-3 gap-y-1.5",
          sheet ? "flex-nowrap" : "flex-wrap sm:flex-nowrap",
        )}
      >
        <span className="flex shrink-0 items-center gap-1.5">
          <DoorOpen
            className={cn(
              "size-4 shrink-0",
              target ? "text-moto-green-strong" : "text-gray-500",
            )}
            aria-hidden
          />
          <FieldLabel id={labelId} className="mb-0">
            Anmelden in
          </FieldLabel>
        </span>
        <div className={sheet ? "min-w-0 flex-1" : "w-full min-w-0 sm:w-80"}>
          <CustomSelect
            value={target?.value ?? WITHOUT_ROOM}
            options={options}
            onChange={state.setTargetValue}
            ariaLabelledBy={labelId}
            ariaDescribedBy={hintId}
            disabled={disabled || state.loading}
            placeholder={state.loading ? "Räume werden geladen …" : undefined}
            searchable={options.length > SEARCH_FROM}
            searchPlaceholder="Raum oder Aktivität suchen"
            testId="checkin-target-select"
          />
        </div>
        <p
          id={hintId}
          className={cn(
            "min-w-0 text-xs text-balance text-gray-600",
            // On the phone the select itself names the target; the line
            // stays for screen readers.
            sheet ? "sr-only" : "w-full sm:w-auto sm:flex-1",
          )}
        >
          {target ? (
            <>
              Kinder sind nach dem Anmelden sofort in{" "}
              <span className="text-moto-green-strong font-semibold">
                {target.roomName}
              </span>
              .
            </>
          ) : (
            "Kinder stehen nach dem Anmelden auf „Unterwegs“, bis sie in einem Raum sind."
          )}
        </p>
      </div>

      {state.lostTargetName ? (
        <Alert
          type="warning"
          className="mt-2"
          announce="polite"
          onDismiss={state.dismissLost}
          message={`In ${state.lostTargetName} läuft keine Aufsicht mehr oder der Raum ist nicht mehr freigegeben. Kinder werden jetzt ohne Raum angemeldet. Wählen Sie ein neues Ziel.`}
        />
      ) : null}
      <LoadErrorAlert
        error={loadError}
        className="mt-2"
        actionLayout="responsive"
      />
    </div>
  );
}
