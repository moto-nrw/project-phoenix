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
  disableHover = false
}: ModernInfoCardProps) {
  return (
    <>
      <div
        className={`group relative overflow-hidden rounded-xl bg-white border border-gray-200 transition-all duration-200 ${!disableHover ? 'hover:shadow-sm' : ''}`}
      >
        {/* Subtle gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-gray-50/20 to-white/50 opacity-50"></div>
        
        {/* Content Container */}
        <div className="relative p-4 sm:p-6">
          {/* Header */}
          <div className="flex items-center gap-3 mb-4">
            {/* Icon Container */}
            <div
              className={`h-10 w-10 rounded-lg ${iconBg} flex items-center justify-center flex-shrink-0`}
            >
              <div className={`${iconColor}`}>
                {icon}
              </div>
            </div>

            <h2 className="text-lg font-semibold text-gray-900">
              {title}
            </h2>
          </div>

          {/* Content */}
          <div className="space-y-3">
            {children}
          </div>
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
        <div className="flex-shrink-0 mt-0.5 sm:mt-1 text-gray-400">
          <div className="h-4 w-4 sm:h-5 sm:w-5">
            {icon}
          </div>
        </div>
      )}
      <div className="flex-1 min-w-0">
        <p className="text-xs sm:text-sm font-medium text-gray-500 mb-1">{label}</p>
        <div className="text-sm sm:text-base text-gray-900 font-semibold break-words leading-relaxed">
          {value}
        </div>
      </div>
    </div>
  );
}