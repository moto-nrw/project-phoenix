"use client";

interface ModernNavCardProps {
  title: string;
  icon: React.ReactNode;
  iconColor: string;
  iconBg: string;
  onClick: () => void;
  index?: number;
}

export function ModernNavCard({ 
  title, 
  icon, 
  iconColor, 
  iconBg,
  onClick, 
  index = 0 
}: ModernNavCardProps) {
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
      
      <button
        className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.05] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.95] w-full touch-manipulation"
        style={{
          transform: `rotate(${(index % 3 - 1) * 0.2}deg)`, // Reduced rotation for mobile
          animation: `float 8s ease-in-out infinite ${index * 0.5}s`
        }}
        onClick={onClick}
        type="button"
      >
        {/* Modern gradient overlay */}
        <div className="absolute inset-0 bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03] rounded-3xl"></div>
        {/* Subtle inner glow */}
        <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
        {/* Modern border highlight */}
        <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
        
        {/* Compact Mobile-First Content Container */}
        <div className="relative p-3 sm:p-4 lg:p-6 flex flex-col items-center text-center min-h-[100px] sm:min-h-[120px] lg:min-h-[140px] justify-center">
          {/* Compact Icon Container */}
          <div 
            className={`relative mb-2 sm:mb-3 h-10 w-10 sm:h-12 sm:w-12 lg:h-14 lg:w-14 rounded-xl ${iconBg} flex items-center justify-center shadow-[0_6px_20px_rgba(80,128,216,0.3)] md:group-hover:shadow-[0_8px_25px_rgba(80,128,216,0.4)] transition-all duration-300`}
          >
            <div className="absolute inset-0 bg-gradient-to-br from-white/20 to-transparent rounded-xl"></div>
            <div className={`relative z-10 ${iconColor}`}>
              <div className="h-5 w-5 sm:h-6 sm:w-6 lg:h-7 lg:w-7">
                {icon}
              </div>
            </div>
            
            {/* Decorative ping */}
            <div className="absolute -top-0.5 -right-0.5 w-2 h-2 sm:w-2.5 sm:h-2.5 bg-white/50 rounded-full animate-ping"></div>
          </div>
          
          {/* Compact Title */}
          <h3 className="text-xs sm:text-sm lg:text-base font-bold text-gray-800 md:group-hover:text-blue-600 transition-colors duration-300 leading-tight px-1 text-center">
            {title}
          </h3>
          
          {/* Subtle Arrow Indicator */}
          <svg className="w-3 h-3 sm:w-4 sm:h-4 text-gray-300 md:group-hover:text-blue-500 md:group-hover:translate-x-0.5 transition-all duration-300 mt-1 sm:mt-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>

          {/* Decorative elements */}
          <div className="absolute top-3 left-3 w-4 h-4 bg-white/20 rounded-full animate-ping"></div>
          <div className="absolute bottom-3 right-3 w-3 h-3 bg-white/30 rounded-full"></div>
        </div>

        {/* Glowing border effect */}
        <div className="absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
      </button>
    </>
  );
}