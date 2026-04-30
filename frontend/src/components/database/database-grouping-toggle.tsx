"use client";

import { ChevronDown, Layers } from "lucide-react";
import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";

interface GroupingOption<K extends string> {
  value: K;
  label: string;
}

interface DatabaseGroupingToggleProps<K extends string> {
  value: K;
  options: GroupingOption<K>[];
  onChange: (next: K) => void;
}

export function DatabaseGroupingToggle<K extends string>({
  value,
  options,
  onChange,
}: DatabaseGroupingToggleProps<K>) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const listboxId = useId();

  const activeIndex = Math.max(
    options.findIndex((option) => option.value === value),
    0,
  );
  const [focusIndex, setFocusIndex] = useState(activeIndex);

  useEffect(() => {
    if (!open) return;
    const handleClick = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  // Move DOM focus to the active option whenever the listbox opens or the
  // user navigates with the keyboard.
  useEffect(() => {
    if (!open) return;
    const target = optionRefs.current[focusIndex];
    target?.focus();
  }, [open, focusIndex]);

  const closeAndReturnFocus = useCallback(() => {
    setOpen(false);
    buttonRef.current?.focus();
  }, []);

  const selectAt = useCallback(
    (index: number) => {
      const option = options[index];
      if (!option) return;
      onChange(option.value);
      closeAndReturnFocus();
    },
    [closeAndReturnFocus, onChange, options],
  );

  const handleTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (
      event.key === "ArrowDown" ||
      event.key === "ArrowUp" ||
      event.key === "Enter" ||
      event.key === " "
    ) {
      event.preventDefault();
      setFocusIndex(activeIndex);
      setOpen(true);
    }
  };

  const handleOptionKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        setFocusIndex((prev) => (prev + 1) % options.length);
        break;
      case "ArrowUp":
        event.preventDefault();
        setFocusIndex((prev) => (prev - 1 + options.length) % options.length);
        break;
      case "Home":
        event.preventDefault();
        setFocusIndex(0);
        break;
      case "End":
        event.preventDefault();
        setFocusIndex(options.length - 1);
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        selectAt(focusIndex);
        break;
      case "Escape":
      case "Tab":
        event.preventDefault();
        closeAndReturnFocus();
        break;
      default:
        break;
    }
  };

  const activeLabel =
    options.find((option) => option.value === value)?.label ??
    options[0]?.label ??
    "";

  return (
    <div ref={containerRef} className="relative">
      <button
        ref={buttonRef}
        type="button"
        onClick={() => {
          setFocusIndex(activeIndex);
          setOpen((prev) => !prev);
        }}
        onKeyDown={handleTriggerKeyDown}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        className="flex h-10 items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        <Layers className="h-4 w-4 text-gray-500" aria-hidden />
        <span className="text-gray-500">Gruppieren:</span>
        <span className="font-semibold text-gray-900">{activeLabel}</span>
        <ChevronDown className="h-3.5 w-3.5 text-gray-500" aria-hidden />
      </button>
      {open ? (
        <ul
          id={listboxId}
          role="listbox"
          className="absolute right-0 z-50 mt-1 w-44 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg"
        >
          {options.map((option, index) => {
            const isActive = option.value === value;
            const isFocused = index === focusIndex;
            return (
              <li key={option.value}>
                <button
                  ref={(el) => {
                    optionRefs.current[index] = el;
                  }}
                  id={`${listboxId}-option-${index}`}
                  type="button"
                  role="option"
                  aria-selected={isActive}
                  tabIndex={isFocused ? 0 : -1}
                  onClick={() => selectAt(index)}
                  onKeyDown={handleOptionKeyDown}
                  onMouseEnter={() => setFocusIndex(index)}
                  className={
                    isActive
                      ? "flex w-full items-center justify-between bg-[#DCF5C1]/60 px-3 py-2 text-left text-sm font-semibold text-gray-900"
                      : "flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
                  }
                >
                  {option.label}
                </button>
              </li>
            );
          })}
        </ul>
      ) : null}
    </div>
  );
}
