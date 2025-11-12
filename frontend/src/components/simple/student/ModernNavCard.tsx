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
  index = 0,
}: ModernNavCardProps) {
  return (
    <>
      {/* Mobile-optimized floating animation with reduced motion support */}
      <style jsx>{`
        @keyframes float {
          0%,
          100% {
            transform: translateY(0px) rotate(${((index % 3) - 1) * 0.2}deg);
          }
          50% {
            transform: translateY(-3px) rotate(${((index % 3) - 1) * 0.2}deg);
          }
        }

        @media (prefers-reduced-motion: reduce) {
          @keyframes float {
            0%,
            100% {
              transform: translateY(0px) rotate(0deg);
            }
            50% {
              transform: translateY(0px) rotate(0deg);
            }
          }
        }
      `}</style>

      <button
        className="group relative w-full cursor-pointer touch-manipulation overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.95] md:hover:-translate-y-3 md:hover:scale-[1.05] md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
        style={{
          transform: `rotate(${((index % 3) - 1) * 0.2}deg)`, // Reduced rotation for mobile
          animation: `float 8s ease-in-out infinite ${index * 0.5}s`,
        }}
        onClick={onClick}
        type="button"
      >
        {/* Modern gradient overlay */}
        <div className="absolute inset-0 rounded-3xl bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03]"></div>
        {/* Subtle inner glow */}
        <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
        {/* Modern border highlight */}
        <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

        {/* Compact Mobile-First Content Container */}
        <div className="relative flex min-h-[100px] flex-col items-center justify-center p-3 text-center sm:min-h-[120px] sm:p-4 lg:min-h-[140px] lg:p-6">
          {/* Compact Icon Container */}
          <div
            className={`relative mb-2 h-10 w-10 rounded-xl sm:mb-3 sm:h-12 sm:w-12 lg:h-14 lg:w-14 ${iconBg} flex items-center justify-center shadow-[0_6px_20px_rgba(80,128,216,0.3)] transition-all duration-300 md:group-hover:shadow-[0_8px_25px_rgba(80,128,216,0.4)]`}
          >
            <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-white/20 to-transparent"></div>
            <div className={`relative z-10 ${iconColor}`}>
              <div className="h-5 w-5 sm:h-6 sm:w-6 lg:h-7 lg:w-7">{icon}</div>
            </div>

            {/* Decorative ping */}
            <div className="absolute -top-0.5 -right-0.5 h-2 w-2 animate-ping rounded-full bg-white/50 sm:h-2.5 sm:w-2.5"></div>
          </div>

          {/* Compact Title */}
          <h3 className="px-1 text-center text-xs leading-tight font-bold text-gray-800 transition-colors duration-300 sm:text-sm md:group-hover:text-blue-600 lg:text-base">
            {title}
          </h3>

          {/* Subtle Arrow Indicator */}
          <svg
            className="mt-1 h-3 w-3 text-gray-300 transition-all duration-300 sm:mt-2 sm:h-4 sm:w-4 md:group-hover:translate-x-0.5 md:group-hover:text-blue-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M9 5l7 7-7 7"
            />
          </svg>

          {/* Decorative elements */}
          <div className="absolute top-3 left-3 h-4 w-4 animate-ping rounded-full bg-white/20"></div>
          <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30"></div>
        </div>

        {/* Glowing border effect */}
        <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
      </button>
    </>
  );
}
