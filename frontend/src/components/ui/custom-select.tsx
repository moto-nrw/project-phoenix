"use client";

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { Check, ChevronDown } from "lucide-react";

export interface CustomSelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
}

interface CustomSelectProps {
  readonly value: string;
  readonly options: readonly CustomSelectOption[];
  readonly onChange: (value: string) => void;
  readonly id?: string;
  readonly name?: string;
  readonly ariaLabel?: string;
  readonly labelId?: string;
  readonly placeholder?: string;
  readonly disabled?: boolean;
  readonly required?: boolean;
  readonly invalid?: boolean;
  readonly className?: string;
  readonly menuClassName?: string;
}

interface DropdownPosition {
  readonly top: number;
  readonly left: number;
  readonly width: number;
  readonly alignRight: boolean;
}

const TRIGGER_BASE_CLASS =
  "flex items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80";

const DEFAULT_TRIGGER_CLASS =
  "moto-content-surface h-10 w-full hover:border-gray-300";

function selectedIndexForValue(
  options: readonly CustomSelectOption[],
  value: string,
): number {
  const index = options.findIndex((option) => option.value === value);
  return index >= 0 ? index : options.findIndex((option) => !option.disabled);
}

function nextEnabledIndex(
  options: readonly CustomSelectOption[],
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

export function CustomSelect({
  value,
  options,
  onChange,
  id,
  name,
  ariaLabel,
  labelId,
  placeholder = "Bitte wählen",
  disabled = false,
  required = false,
  invalid = false,
  className = "",
  menuClassName = "",
}: CustomSelectProps) {
  const generatedId = useId();
  const triggerId = id ?? generatedId;
  const listboxId = `${triggerId}-listbox`;
  const selectedOption = useMemo(
    () => options.find((option) => option.value === value),
    [options, value],
  );
  const [isOpen, setIsOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const [activeIndex, setActiveIndex] = useState(() =>
    selectedIndexForValue(options, value),
  );
  const [position, setPosition] = useState<DropdownPosition>({
    top: 0,
    left: 0,
    width: 0,
    alignRight: false,
  });
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!isOpen) return;
    setActiveIndex(selectedIndexForValue(options, value));
  }, [isOpen, options, value]);

  const close = useCallback(() => setIsOpen(false), []);

  const updatePosition = useCallback(() => {
    const button = buttonRef.current;
    if (!button) return;

    const rect = button.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const width = Math.max(rect.width, 192);
    const alignRight = rect.left + width > viewportWidth - 16;

    setPosition({
      top: rect.bottom + 4,
      left: alignRight ? rect.right : rect.left,
      width,
      alignRight,
    });
  }, []);

  useEffect(() => {
    if (!isOpen) return;
    updatePosition();

    function handlePointerDown(event: MouseEvent) {
      const target = event.target as Node;
      if (
        !buttonRef.current?.contains(target) &&
        !menuRef.current?.contains(target)
      ) {
        close();
      }
    }

    function handleScrollOrResize() {
      updatePosition();
    }

    document.addEventListener("mousedown", handlePointerDown);
    window.addEventListener("scroll", handleScrollOrResize, true);
    window.addEventListener("resize", handleScrollOrResize);

    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      window.removeEventListener("scroll", handleScrollOrResize, true);
      window.removeEventListener("resize", handleScrollOrResize);
    };
  }, [close, isOpen, updatePosition]);

  const selectOption = useCallback(
    (option: CustomSelectOption | undefined) => {
      if (!option || option.disabled) return;
      onChange(option.value);
      close();
    },
    [close, onChange],
  );

  const moveActive = useCallback(
    (direction: 1 | -1) => {
      setActiveIndex((current) =>
        nextEnabledIndex(options, current < 0 ? 0 : current, direction),
      );
    },
    [options],
  );

  const handleKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return;

    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!isOpen) {
        setIsOpen(true);
        return;
      }
      moveActive(event.key === "ArrowDown" ? 1 : -1);
      return;
    }

    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (!isOpen) {
        setIsOpen(true);
        return;
      }
      selectOption(options[activeIndex]);
      return;
    }

    if (event.key === "Escape") {
      event.preventDefault();
      close();
    }
  };

  const menu = isOpen && mounted && (
    <div
      ref={menuRef}
      id={listboxId}
      role="listbox"
      aria-labelledby={labelId}
      className={`fixed z-[9999] max-h-72 overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg ${menuClassName}`}
      style={{
        top: position.top,
        left: position.alignRight ? "auto" : position.left,
        right: position.alignRight ? window.innerWidth - position.left : "auto",
        minWidth: position.width,
      }}
    >
      {options.map((option, index) => {
        const selected = option.value === value;
        const active = index === activeIndex;

        return (
          <button
            key={option.value}
            type="button"
            role="option"
            aria-selected={selected}
            disabled={option.disabled}
            onMouseEnter={() => setActiveIndex(index)}
            onClick={() => selectOption(option)}
            className={`flex w-full items-center gap-2 px-4 py-2 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:text-gray-400 ${
              selected || active
                ? "bg-gray-50 font-medium text-gray-900"
                : "text-gray-700 hover:bg-gray-50"
            }`}
          >
            <span className="min-w-0 flex-1 truncate">{option.label}</span>
            {selected ? (
              <Check className="h-4 w-4 flex-shrink-0 text-gray-600" />
            ) : null}
          </button>
        );
      })}
    </div>
  );

  return (
    <>
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <button
        ref={buttonRef}
        id={triggerId}
        type="button"
        role="combobox"
        aria-controls={listboxId}
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-label={ariaLabel}
        aria-labelledby={labelId}
        aria-required={required || undefined}
        aria-invalid={invalid || undefined}
        disabled={disabled}
        onClick={() => {
          if (!disabled) setIsOpen((open) => !open);
        }}
        onKeyDown={handleKeyDown}
        className={`${TRIGGER_BASE_CLASS} ${DEFAULT_TRIGGER_CLASS} ${
          invalid ? "border-[#FF3130] bg-[#FF3130]/5" : ""
        } ${className}`}
      >
        <span
          className={`min-w-0 flex-1 truncate ${
            selectedOption && value !== "" ? "text-gray-900" : "text-gray-500"
          }`}
        >
          {selectedOption?.label ?? placeholder}
        </span>
        <ChevronDown
          aria-hidden="true"
          className={`h-4 w-4 flex-shrink-0 text-gray-400 transition-transform ${
            isOpen ? "rotate-180" : ""
          }`}
        />
      </button>
      {mounted ? createPortal(menu, document.body) : null}
    </>
  );
}
