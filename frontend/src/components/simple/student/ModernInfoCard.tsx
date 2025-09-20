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
  index = 0,
  disableHover = false
}: ModernInfoCardProps) {
  return (
    <>
      {/* Mobile-optimized floating animation with reduced motion support */}
      <style jsx>{`
        @keyframes float {
          0%, 100% { transform: translateY(0px) rotate(${(index % 3 - 1) * 0.2}deg); }
          50% { transform: translateY(-3px) rotate(${(index % 3 - 1) * 0.2}deg); }
        }
        
        @media (prefers-reduced-motion: reduce) {
          @keyframes float {
            0%, 100% { transform: translateY(0px) rotate(0deg); }
            50% { transform: translateY(0px) rotate(0deg); }
          }
        }
      `}</style>
      
      <div 
        className={`group relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 ${!disableHover ? 'md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white' : ''}`}
        style={{
          transform: `rotate(${(index % 3 - 1) * 0.2}deg)`, // Reduced rotation for mobile
          animation: `float 8s ease-in-out infinite ${index * 0.7}s`
        }}
      >
        {/* Modern gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03] rounded-3xl"></div>
        {/* Subtle inner glow */}
        <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
        {/* Modern border highlight */}
        <div className={`absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 ${!disableHover ? 'md:group-hover:ring-blue-200/60' : ''}`}></div>
        
        {/* Mobile-Optimized Content Container */}
        <div className="relative p-4 sm:p-6 lg:p-8">
          {/* Mobile-Optimized Header */}
          <div className="flex items-center gap-3 sm:gap-4 mb-4 sm:mb-6">
            {/* Mobile-Optimized Icon Container */}
            <div 
              className={`relative h-10 w-10 sm:h-12 sm:w-12 rounded-2xl ${iconBg} flex items-center justify-center shadow-[0_8px_25px_rgba(80,128,216,0.3)] flex-shrink-0`}
            >
              <div className="absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-2xl"></div>
              <div className={`relative z-10 ${iconColor}`}>
                {icon}
              </div>
            </div>
            
            <h2 className={`text-lg sm:text-xl font-bold text-gray-800 transition-colors duration-300 leading-tight ${!disableHover ? 'md:group-hover:text-blue-600' : ''}`}>
              {title}
            </h2>
          </div>

          {/* Mobile-Optimized Content */}
          <div className="space-y-4 sm:space-y-6">
            {children}
          </div>

          {/* Decorative elements */}
          <div className="absolute top-4 left-4 w-4 h-4 bg-white/20 rounded-full animate-ping"></div>
          <div className="absolute bottom-4 right-4 w-3 h-3 bg-white/30 rounded-full"></div>
        </div>

        {/* Glowing border effect */}
        <div className={`absolute inset-0 rounded-3xl opacity-0 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent ${!disableHover ? 'md:group-hover:opacity-100' : ''}`}></div>
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