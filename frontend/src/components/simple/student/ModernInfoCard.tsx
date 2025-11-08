"use client";

interface ModernInfoCardProps {
  title: string;
  children: React.ReactNode;
  icon: React.ReactNode;
  iconColor: string;
  iconBg: string;
  index?: number;
  disableHover?: boolean;
}

export function ModernInfoCard({
  title,
  children,
  icon,
  iconColor,
  iconBg,
  index: _index = 0,
  disableHover = false,
}: ModernInfoCardProps) {
  return (
    <>
      <div
        className={`group relative overflow-hidden rounded-xl border border-gray-200 bg-white transition-all duration-200 ${!disableHover ? "hover:shadow-sm" : ""}`}
      >
        {/* Subtle gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-gray-50/20 to-white/50 opacity-50"></div>

        {/* Content Container */}
        <div className="relative p-4 sm:p-6">
          {/* Header */}
          <div className="mb-4 flex items-center gap-3">
            {/* Icon Container */}
            <div
              className={`h-10 w-10 rounded-lg ${iconBg} flex flex-shrink-0 items-center justify-center`}
            >
              <div className={`${iconColor}`}>{icon}</div>
            </div>

            <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
          </div>

          {/* Content */}
          <div className="space-y-3">{children}</div>
        </div>
      </div>
    </>
  );
}

interface ModernInfoItemProps {
  label: string;
  value: string | React.ReactNode;
  icon?: React.ReactNode;
}

export function ModernInfoItem({ label, value, icon }: ModernInfoItemProps) {
  return (
    <div className="flex items-start gap-3 sm:gap-4">
      {icon && (
        <div className="mt-0.5 flex-shrink-0 text-gray-400 sm:mt-1">
          <div className="h-4 w-4 sm:h-5 sm:w-5">{icon}</div>
        </div>
      )}
      <div className="min-w-0 flex-1">
        <p className="mb-1 text-xs font-medium text-gray-500 sm:text-sm">
          {label}
        </p>
        <div className="text-sm leading-relaxed font-semibold break-words text-gray-900 sm:text-base">
          {value}
        </div>
      </div>
    </div>
  );
}
