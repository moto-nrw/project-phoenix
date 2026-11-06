"use client";

import { Minus, Plus } from "lucide-react";
import type { InputHTMLAttributes } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";

/**
 * Zahlenfeld mit Minus und Plus. Die Rechnung bleibt bei der Stelle, die den
 * Baustein nutzt: sie kennt Grenzen und Schrittweite und meldet über
 * `canDecrement` und `canIncrement`, ob ein Schritt noch geht.
 *
 * Der Pausen-Stepper der Zeiterfassung ist bewusst nicht dieser Baustein
 * (`.claude/rules/frontend-ui-kit.md`).
 */
export function NumberStepper({
  value,
  onChange,
  onStep,
  decrementLabel,
  incrementLabel,
  canDecrement = true,
  canIncrement = true,
  disabled = false,
  error,
  ...inputProps
}: Readonly<
  Omit<
    InputHTMLAttributes<HTMLInputElement>,
    "value" | "onChange" | "className" | "size"
  > & {
    id: string;
    name: string;
    value: string;
    onChange: NonNullable<InputHTMLAttributes<HTMLInputElement>["onChange"]>;
    onStep: (delta: -1 | 1) => void;
    decrementLabel: string;
    incrementLabel: string;
    canDecrement?: boolean;
    canIncrement?: boolean;
    disabled?: boolean;
    error?: string;
  }
>) {
  return (
    <div className="flex items-start gap-2">
      <Button
        type="button"
        variant="outline"
        size="icon"
        className="mt-1 pointer-coarse:mt-0"
        onClick={() => onStep(-1)}
        disabled={disabled || !canDecrement}
        aria-label={decrementLabel}
      >
        <Minus className="h-4 w-4" aria-hidden="true" />
      </Button>
      <div className="w-24">
        <Input
          {...inputProps}
          controlSize="compact"
          className="[appearance:textfield] text-center tabular-nums [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
          value={value}
          onChange={onChange}
          disabled={disabled}
          error={error}
        />
      </div>
      <Button
        type="button"
        variant="outline"
        size="icon"
        className="mt-1 pointer-coarse:mt-0"
        onClick={() => onStep(1)}
        disabled={disabled || !canIncrement}
        aria-label={incrementLabel}
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
      </Button>
    </div>
  );
}
