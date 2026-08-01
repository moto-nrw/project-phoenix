import { Children, isValidElement } from "react";
import { Skeleton } from "~/components/ui/skeleton";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState as UIEmptyState } from "~/components/ui/empty-state";

export function FormField({
  label,
  htmlFor,
  required,
  children,
}: {
  readonly label: string;
  readonly htmlFor: string;
  readonly required?: boolean;
  readonly children: React.ReactNode;
}) {
  return (
    <div>
      <label
        id={`${htmlFor}-label`}
        htmlFor={htmlFor}
        className="mb-1 block text-sm font-medium text-gray-700"
      >
        {label}
        {required && <span className="ml-0.5 text-red-500">*</span>}
      </label>
      {children}
    </div>
  );
}

export function FormError({
  message,
  ref,
}: {
  readonly message: string;
  readonly ref?: React.Ref<HTMLDivElement>;
}) {
  return (
    <div
      ref={ref}
      role="alert"
      className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600"
    >
      {message}
    </div>
  );
}

export function FieldWarning({ message }: { readonly message: string }) {
  return (
    <p className="mt-1 rounded bg-amber-50 px-2 py-1 text-xs text-amber-700">
      {message}
    </p>
  );
}

export function StatusBadge({ active }: { readonly active: boolean }) {
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-xs font-medium ${
        active ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"
      }`}
    >
      {active ? "Aktiv" : "Inaktiv"}
    </span>
  );
}

export function DeliveryStatusBadge({ status }: { readonly status: string }) {
  const styles: Record<string, string> = {
    sent: "bg-green-100 text-green-700",
    pending: "bg-yellow-100 text-yellow-700",
    failed: "bg-red-100 text-red-700",
  };
  const labels: Record<string, string> = {
    sent: "Gesendet",
    pending: "Ausstehend",
    failed: "Fehlgeschlagen",
  };
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-500"}`}
    >
      {labels[status] ?? status}
    </span>
  );
}

export function EmptyState({
  title,
  description,
  buttonLabel,
  onAction,
}: {
  readonly title: string;
  readonly description: string;
  readonly buttonLabel: string;
  readonly onAction: () => void;
}) {
  return (
    <UIEmptyState
      icon={
        <svg
          className="h-12 w-12"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={1.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0012 9.75c-2.551 0-5.056.2-7.5.582V21"
          />
        </svg>
      }
      title={title}
      description={description}
      action={
        <button
          type="button"
          onClick={onAction}
          className="rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
        >
          {buttonLabel}
        </button>
      }
    />
  );
}

export function SimpleEmptyState({
  title,
  description,
}: {
  readonly title: string;
  readonly description: string;
}) {
  return <UIEmptyState title={title} description={description} />;
}

export function SelectWithChevron({
  children,
  value,
  onChange,
  id,
  name,
  disabled,
  required,
  "aria-label": ariaLabel,
}: React.SelectHTMLAttributes<HTMLSelectElement> & {
  readonly children: React.ReactNode;
}) {
  const options = Children.toArray(children)
    .filter(isValidElement)
    .map((child) => {
      const optionProps = (
        child as React.ReactElement<
          React.OptionHTMLAttributes<HTMLOptionElement>
        >
      ).props;
      return {
        value: optionProps.value != null ? String(optionProps.value) : "",
        label:
          typeof optionProps.children === "string"
            ? optionProps.children
            : String(optionProps.children ?? ""),
        disabled: optionProps.disabled,
      };
    });

  // Without an explicit aria-label, name the field (trigger AND popup listbox)
  // from the associated FormField <label>, which carries id `${id}-label`.
  // CustomSelect forwards ariaLabelledBy to the popup, which cannot be reached
  // by the label's htmlFor the way the trigger button is.
  const ariaLabelledBy = !ariaLabel && id ? `${id}-label` : undefined;

  return (
    <CustomSelect
      id={id}
      name={name}
      value={value != null ? String(value) : ""}
      options={options}
      onChange={(next) =>
        onChange?.({
          target: { value: next },
        } as unknown as React.ChangeEvent<HTMLSelectElement>)
      }
      disabled={disabled}
      required={required}
      ariaLabel={ariaLabel}
      ariaLabelledBy={ariaLabelledBy}
    />
  );
}

export function PlusIcon() {
  return (
    <svg
      className="h-5 w-5"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
    </svg>
  );
}

export function VisibilityToggle({
  hidden,
  onToggle,
}: {
  readonly hidden: boolean;
  readonly onToggle: () => void;
}) {
  return (
    <div className="border-t border-gray-100 pt-4">
      <p className="mb-3 text-xs font-medium text-gray-500 uppercase">
        Sichtbarkeit
      </p>
      <div className="flex items-center justify-between rounded-lg border border-gray-200 px-3 py-2">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-gray-900">
            {hidden ? "Verborgen" : "Sichtbar"}
          </p>
          <p className="text-xs text-gray-500">
            {hidden
              ? "Schule wird nicht auf der Startseite angezeigt, ist aber über den Direktlink erreichbar."
              : "Schule wird auf der Startseite im Schulwähler angezeigt."}
          </p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={!hidden}
          aria-label="Sichtbarkeit umschalten"
          onClick={onToggle}
          className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:outline-none ${
            hidden ? "bg-gray-300" : "bg-blue-600"
          }`}
        >
          <span
            className={`pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
              hidden ? "translate-x-0" : "translate-x-5"
            }`}
          />
        </button>
      </div>
    </div>
  );
}

export function CardSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-sm"
        >
          <div className="space-y-3">
            <Skeleton className="h-5 w-3/5 rounded" />
            <Skeleton className="h-4 w-2/5 rounded" />
            <Skeleton className="h-3 w-1/4 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
