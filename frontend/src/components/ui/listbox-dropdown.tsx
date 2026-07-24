"use client";

import { ChevronDown } from "lucide-react";
import {
  useCallback,
  type CSSProperties,
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";

export interface ListboxDropdownOption<K extends string> {
  readonly value: K;
  readonly label: string;
  readonly disabled?: boolean;
}

interface ListboxDropdownProps<K extends string> {
  readonly value: K;
  readonly options: readonly ListboxDropdownOption<K>[];
  readonly onChange: (next: K) => void;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly containerClassName?: string;
  readonly containerStyle?: CSSProperties;
  readonly className?: string;
  readonly menuClassName?: string;
  readonly optionClassName?: string;
  readonly activeOptionClassName?: string;
  readonly disabledOptionClassName?: string;
  readonly disabled?: boolean;
  readonly required?: boolean;
  readonly invalid?: boolean;
  readonly placeholder?: string;
  readonly triggerRole?: "button" | "combobox";
  readonly testId?: string;
  readonly renderTrigger?: (args: {
    selectedLabel: string;
    open: boolean;
  }) => ReactNode;
}

function firstEnabledIndex<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
): number {
  return Math.max(
    options.findIndex((option) => !option.disabled),
    0,
  );
}

function selectedIndexForValue<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
  value: K,
): number {
  const selectedIndex = options.findIndex((option) => option.value === value);
  return selectedIndex >= 0 ? selectedIndex : firstEnabledIndex(options);
}

function nextEnabledIndex<K extends string>(
  options: readonly ListboxDropdownOption<K>[],
  startIndex: number,
  direction: 1 | -1,
): number {
  if (options.length === 0) return -1;

  for (let offset = 1; offset <= options.length; offset++) {
    const index =
      (startIndex + direction * offset + options.length) % options.length;
    if (!options[index]?.disabled) return index;
  }

  return -1;
}

function consumeEscape(event: KeyboardEvent<HTMLButtonElement>): void {
  event.preventDefault();
  event.stopPropagation();
  event.nativeEvent.stopImmediatePropagation();
}

export function ListboxDropdown<K extends string>({
  value,
  options,
  onChange,
  id,
  ariaLabel,
  containerClassName = "relative",
  containerStyle,
  className = "",
  menuClassName = "",
  optionClassName = "",
  activeOptionClassName = "",
  disabledOptionClassName = "",
  disabled = false,
  required = false,
  invalid = false,
  placeholder = "Bitte wählen",
  triggerRole = "button",
  testId,
  renderTrigger,
}: ListboxDropdownProps<K>) {
  const generatedListboxId = useId();
  const listboxId = id ? `${id}-listbox` : generatedListboxId;
  const containerRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const optionRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const [open, setOpen] = useState(false);
  const selectedIndex = selectedIndexForValue(options, value);
  const [focusIndex, setFocusIndex] = useState(selectedIndex);
  const selectedOption = options.find((option) => option.value === value);
  const selectedLabel = selectedOption?.label ?? placeholder;

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

  useEffect(() => {
    if (!open) return;
    optionRefs.current[focusIndex]?.focus();
  }, [open, focusIndex]);

  const closeAndReturnFocus = useCallback(() => {
    setOpen(false);
    buttonRef.current?.focus();
  }, []);

  const selectAt = useCallback(
    (index: number) => {
      const option = options[index];
      if (!option || option.disabled) return;
      onChange(option.value);
      closeAndReturnFocus();
    },
    [closeAndReturnFocus, onChange, options],
  );

  const openAtSelected = () => {
    setFocusIndex(selectedIndex);
    setOpen(true);
  };

  const handleTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return;
    if (event.key === "Escape" && open) {
      consumeEscape(event);
      setOpen(false);
      return;
    }
    if (open) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setFocusIndex((prev) => nextEnabledIndex(options, prev, 1));
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        setFocusIndex((prev) => nextEnabledIndex(options, prev, -1));
        return;
      }
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        selectAt(focusIndex);
        return;
      }
    }
    if (
      event.key === "ArrowDown" ||
      event.key === "ArrowUp" ||
      event.key === "Enter" ||
      event.key === " "
    ) {
      event.preventDefault();
      openAtSelected();
    }
  };

  const handleOptionKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        setFocusIndex((prev) => nextEnabledIndex(options, prev, 1));
        break;
      case "ArrowUp":
        event.preventDefault();
        setFocusIndex((prev) => nextEnabledIndex(options, prev, -1));
        break;
      case "Home":
        event.preventDefault();
        setFocusIndex(firstEnabledIndex(options));
        break;
      case "End":
        event.preventDefault();
        for (let index = options.length - 1; index >= 0; index--) {
          if (!options[index]?.disabled) {
            setFocusIndex(index);
            break;
          }
        }
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        selectAt(focusIndex);
        break;
      case "Escape":
        consumeEscape(event);
        closeAndReturnFocus();
        break;
      case "Tab":
        // Close and move focus back to the trigger WITHOUT cancelling the
        // default action: the browser then continues the Tab traversal
        // from the trigger, so keyboard users reach the next field instead
        // of being trapped inside the popup.
        closeAndReturnFocus();
        break;
      default:
        break;
    }
  };

  return (
    <div
      ref={containerRef}
      className={containerClassName}
      style={containerStyle}
    >
      <button
        ref={buttonRef}
        id={id}
        type="button"
        onClick={(event) => {
          event.preventDefault();
          setFocusIndex(selectedIndex);
          setOpen((prev) => !prev);
        }}
        onKeyDown={handleTriggerKeyDown}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listboxId : undefined}
        aria-label={ariaLabel}
        aria-required={required || undefined}
        aria-invalid={invalid || undefined}
        role={triggerRole === "combobox" ? "combobox" : undefined}
        disabled={disabled}
        className={className}
        data-testid={testId}
      >
        {renderTrigger ? (
          renderTrigger({ selectedLabel, open })
        ) : (
          <>
            <span>{selectedLabel}</span>
            <ChevronDown className="h-4 w-4 text-gray-400" aria-hidden />
          </>
        )}
      </button>
      {open ? (
        <ul
          id={listboxId}
          role="listbox"
          className={menuClassName}
          aria-label={ariaLabel}
        >
          {options.map((option, index) => {
            const isActive = option.value === value;
            const isFocused = index === focusIndex;
            return (
              // The option role lives on the inner button; the list item
              // is pure structure and must not surface in the a11y tree.
              <li key={option.value} role="presentation">
                <button
                  ref={(el) => {
                    optionRefs.current[index] = el;
                  }}
                  id={`${listboxId}-option-${index}`}
                  type="button"
                  role="option"
                  aria-label={option.label}
                  aria-selected={isActive}
                  disabled={option.disabled}
                  tabIndex={isFocused ? 0 : -1}
                  onClick={() => selectAt(index)}
                  onKeyDown={handleOptionKeyDown}
                  onMouseEnter={() => {
                    if (!option.disabled) setFocusIndex(index);
                  }}
                  className={
                    option.disabled
                      ? disabledOptionClassName
                      : isActive || isFocused
                        ? activeOptionClassName
                        : optionClassName
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
