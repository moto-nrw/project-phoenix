import Link from "next/link";

export interface DatabasePageHeaderProps {
  title: string;
  description: string;
  backUrl?: string;
  className?: string;
}

export function DatabasePageHeader({
  title,
  description,
  backUrl,
  className = "",
}: Readonly<DatabasePageHeaderProps>) {
  return (
    <>
      {backUrl && (
        <div className="mb-6">
          <Link
            href={backUrl}
            className="flex items-center text-gray-600 transition-colors hover:text-blue-600"
          >
            <svg
              className="mr-1 h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M10 19l-7-7m0 0l7-7m-7 7h18"
              />
            </svg>
            Zurück
          </Link>
        </div>
      )}
      <div className={`mb-4 md:mb-6 lg:mb-8 ${className}`}>
        <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
          {title}
        </h1>
        <p className="mt-1 text-sm text-gray-600 md:text-base">{description}</p>
      </div>
    </>
  );
}
