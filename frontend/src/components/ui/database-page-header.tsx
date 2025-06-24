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
  className = "" 
}: DatabasePageHeaderProps) {
  return (
    <div className={`mb-4 md:mb-6 lg:mb-8 ${className}`}>
      <div className="flex items-center justify-between">
        <div className="flex-grow">
          <h1 className="text-2xl md:text-3xl font-bold text-gray-900">{title}</h1>
          <p className="mt-1 text-sm md:text-base text-gray-600">{description}</p>
        </div>
        {backUrl && (
          <Link 
            href={backUrl}
            className="ml-4 inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-colors duration-200 touch-manipulation min-h-[44px]"
          >
            <svg
              className="w-4 h-4"
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
            <span className="hidden sm:inline">Zurück</span>
          </Link>
        )}
      </div>
    </div>
  );
}