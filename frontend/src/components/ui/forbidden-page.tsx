"use client";

interface ForbiddenPageProps {
  readonly title?: string;
  readonly message?: string;
}

export function ForbiddenPage({
  title = "Kein Zugriff",
  message = "Du verfügst nicht über die notwendigen Berechtigungen, um diese Seite aufzurufen.",
}: ForbiddenPageProps) {
  return (
    <div className="mx-auto max-w-2xl">
      <div className="border-moto-red/10 bg-moto-red-soft/50 rounded-2xl border p-6">
        <div className="flex items-start gap-3">
          <svg
            className="text-moto-red mt-0.5 h-5 w-5 flex-shrink-0"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
          <div>
            <h3 className="text-moto-red-strong font-semibold">{title}</h3>
            <p className="text-moto-red-strong mt-1 text-sm">{message}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
