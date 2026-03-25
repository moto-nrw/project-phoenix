import { Skeleton } from "~/components/ui/skeleton";

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
    <div className="flex flex-col items-center gap-3 py-12 text-center">
      <svg
        className="h-12 w-12 text-gray-400"
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
      <p className="text-lg font-medium text-gray-900">{title}</p>
      <p className="text-sm text-gray-500">{description}</p>
      <button
        type="button"
        onClick={onAction}
        className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
      >
        {buttonLabel}
      </button>
    </div>
  );
}

export function SimpleEmptyState({
  title,
  description,
}: {
  readonly title: string;
  readonly description: string;
}) {
  return (
    <div className="flex flex-col items-center gap-3 py-12 text-center">
      <p className="text-lg font-medium text-gray-900">{title}</p>
      <p className="text-sm text-gray-500">{description}</p>
    </div>
  );
}

export function SelectWithChevron({
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement> & {
  readonly children: React.ReactNode;
}) {
  return (
    <div className="relative">
      <select
        {...props}
        className="w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm text-gray-900 transition-colors focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500"
      >
        {children}
      </select>
      <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
        <svg
          className="h-4 w-4 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </div>
    </div>
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

export function CardSkeletons() {
  return (
    <div className="mt-4 space-y-4">
      {Array.from({ length: 3 }, (_, i) => (
        <div
          key={i}
          className="rounded-3xl border border-gray-100/50 bg-white/90 p-5 shadow-[0_8px_30px_rgb(0,0,0,0.12)]"
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
