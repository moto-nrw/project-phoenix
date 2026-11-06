"use client";

import {
  type KeyboardEvent,
  type RefObject,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import { cn } from "~/lib/utils";

const HOURS = Array.from({ length: 24 }, (_, hour) =>
  String(hour).padStart(2, "0"),
);
const MINUTES = Array.from({ length: 12 }, (_, step) =>
  String(step * 5).padStart(2, "0"),
);
const COMPLETE_TIME = /^([01]\d|2[0-3]):([0-5]\d)$/;
const ENABLED_OPTION = '[role="option"]:not([aria-disabled="true"])';

function focusColumn(column: HTMLElement | null) {
  const option =
    column?.querySelector<HTMLElement>(
      `${ENABLED_OPTION}[aria-selected="true"]`,
    ) ?? column?.querySelector<HTMLElement>(ENABLED_OPTION);
  option?.focus();
}

function centerSelectedOption(column: HTMLElement | null) {
  const option =
    column?.querySelector<HTMLElement>('[aria-selected="true"]') ??
    column?.querySelector<HTMLElement>(ENABLED_OPTION);
  if (!column || !option) return;
  column.scrollTop =
    option.offsetTop - (column.clientHeight - option.offsetHeight) / 2;
}

function TimeColumn({
  label,
  options,
  selected,
  isDisabled,
  onPick,
  onNavigate,
  columnRef,
  initialOptionRef,
}: Readonly<{
  label: string;
  options: readonly string[];
  selected: string | null;
  isDisabled: (option: string) => boolean;
  onPick: (option: string) => void;
  onNavigate: (event: KeyboardEvent<HTMLElement>) => void;
  columnRef: RefObject<HTMLUListElement | null>;
  initialOptionRef?: RefObject<HTMLElement | null>;
}>) {
  const labelId = useId();
  const initialOption =
    selected !== null && !isDisabled(selected)
      ? selected
      : options.find((option) => !isDisabled(option));

  return (
    <div className="min-w-0 flex-1">
      <p
        id={labelId}
        className="px-1 pb-1 text-center text-xs font-medium text-gray-500"
      >
        {label}
      </p>
      <ul
        ref={columnRef}
        role="listbox"
        aria-labelledby={labelId}
        className="relative h-56 scrollbar-thin space-y-0.5 overflow-y-auto overscroll-contain"
      >
        {options.map((option) => {
          const disabled = isDisabled(option);
          const isSelected = option === selected;
          return (
            <li
              key={option}
              ref={
                option === initialOption
                  ? (initialOptionRef as RefObject<HTMLLIElement | null>)
                  : undefined
              }
              role="option"
              aria-selected={isSelected}
              aria-disabled={disabled || undefined}
              tabIndex={-1}
              onClick={() => {
                if (!disabled) onPick(option);
              }}
              onKeyDown={(event) => {
                if (event.key !== "Enter" && event.key !== " ") {
                  onNavigate(event);
                  return;
                }
                event.preventDefault();
                event.stopPropagation();
                if (!disabled) onPick(option);
              }}
              className={cn(
                "flex h-9 items-center justify-center rounded-lg text-sm tabular-nums focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset pointer-coarse:h-11",
                isSelected && "bg-gray-900 text-white",
                !isSelected && disabled && "text-gray-300",
                !isSelected && !disabled && "text-gray-700 hover:bg-gray-100",
                disabled ? "cursor-not-allowed" : "cursor-pointer",
              )}
            >
              {option}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * Zwei Spalten für `TimeField`: Stunden 00 bis 23, Minuten in 5er-Schritten.
 * Eine andere Minute geht weiter über das Tippen; steht so ein Wert im Feld,
 * ist in der Minutenspalte nichts markiert.
 */
export function TimePickerPanel({
  value,
  minTime,
  onChange,
  close,
  initialFocusRef,
}: Readonly<{
  value: string;
  /** Frühester wählbarer Wert als "HH:MM". */
  minTime?: string;
  onChange: (value: string) => void;
  close: () => void;
  initialFocusRef: RefObject<HTMLElement | null>;
}>) {
  const hourColumnRef = useRef<HTMLUListElement>(null);
  const minuteColumnRef = useRef<HTMLUListElement>(null);
  // Eine Minute vor der Stunde: sie wartet, bis die Stunde die Zeit vollständig macht.
  const [waitingMinute, setWaitingMinute] = useState<string | null>(null);
  const [columnToFocus, setColumnToFocus] = useState<{
    column: "hour" | "minute";
  } | null>(null);

  const time = COMPLETE_TIME.exec(value);
  const hour = time?.[1] ?? null;
  const minute = time?.[2] ?? waitingMinute;

  useLayoutEffect(() => {
    centerSelectedOption(hourColumnRef.current);
    centerSelectedOption(minuteColumnRef.current);
  }, []);

  // Radix hört im Slide-over schon in der Capture-Phase am Dokument auf Escape
  // und würde es mitschließen. Nur das Fenster kommt davor.
  useEffect(() => {
    const closeOnEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopImmediatePropagation();
      close();
    };
    window.addEventListener("keydown", closeOnEscape, true);
    return () => window.removeEventListener("keydown", closeOnEscape, true);
  }, [close]);

  useEffect(() => {
    if (!columnToFocus) return;
    focusColumn(
      columnToFocus.column === "hour"
        ? hourColumnRef.current
        : minuteColumnRef.current,
    );
  }, [columnToFocus]);

  // Ein Wert unter dem Minimum entsteht nie aus der Auswahl: Wer die Stunde der
  // Grenze wählt, landet mit einer zu frühen Minute auf dem Minimum selbst.
  const emit = (nextHour: string, nextMinute: string) => {
    const next = `${nextHour}:${nextMinute}`;
    onChange(minTime && next < minTime ? minTime : next);
  };

  const handleNavigate = (event: KeyboardEvent<HTMLElement>) => {
    const current = event.currentTarget;
    const column = current.closest<HTMLElement>('[role="listbox"]');
    if (!column) return;

    const enabled = Array.from(
      column.querySelectorAll<HTMLElement>(ENABLED_OPTION),
    );
    const index = enabled.indexOf(current);
    const otherColumn =
      column === hourColumnRef.current
        ? minuteColumnRef.current
        : hourColumnRef.current;
    const targets: Record<string, () => void> = {
      ArrowDown: () =>
        enabled[Math.min(index + 1, enabled.length - 1)]?.focus(),
      ArrowUp: () => enabled[Math.max(index - 1, 0)]?.focus(),
      Home: () => enabled[0]?.focus(),
      End: () => enabled.at(-1)?.focus(),
      ArrowLeft: () => focusColumn(otherColumn),
      ArrowRight: () => focusColumn(otherColumn),
      // Die Auswahl hält den Fokus, auch außerhalb eines Dialogs.
      Tab: () => focusColumn(otherColumn),
    };
    const move = targets[event.key];
    if (!move) return;
    event.preventDefault();
    event.stopPropagation();
    move();
  };

  return (
    <div className="flex gap-1 p-2">
      <TimeColumn
        label="Stunde"
        options={HOURS}
        selected={hour}
        isDisabled={(option) =>
          minTime !== undefined && `${option}:59` < minTime
        }
        onPick={(nextHour) => {
          emit(nextHour, minute ?? "00");
          if (waitingMinute !== null && time === null) {
            close();
            return;
          }
          setColumnToFocus({ column: "minute" });
        }}
        onNavigate={handleNavigate}
        columnRef={hourColumnRef}
        initialOptionRef={initialFocusRef}
      />
      <TimeColumn
        label="Minute"
        options={MINUTES}
        selected={minute}
        isDisabled={(option) =>
          minTime !== undefined &&
          hour !== null &&
          `${hour}:${option}` < minTime
        }
        onPick={(nextMinute) => {
          if (hour === null) {
            setWaitingMinute(nextMinute);
            setColumnToFocus({ column: "hour" });
            return;
          }
          emit(hour, nextMinute);
          close();
        }}
        onNavigate={handleNavigate}
        columnRef={minuteColumnRef}
      />
    </div>
  );
}
