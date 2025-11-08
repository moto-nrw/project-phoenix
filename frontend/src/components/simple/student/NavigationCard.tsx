"use client";

interface NavigationCardProps {
  title: string;
  icon: React.ReactNode;
  iconColor: string;
  _href: string; // Prefix with underscore since it's not used but may be needed for future routing
  onClick: () => void;
  index?: number;
}

export function NavigationCard({
  title,
  icon,
  iconColor,
  _href,
  onClick,
  index = 0,
}: NavigationCardProps) {
  // Floating animation style from ogs_groups pattern
  const floatingStyle = {
    animation: `float 8s ease-in-out infinite ${index * 0.5}s`,
    transform: `rotate(${((index % 3) - 1) * 0.2}deg)`,
  };

  return (
    <>
      <style jsx>{`
        @keyframes float {
          0%,
          100% {
            transform: translateY(0px) rotate(${((index % 3) - 1) * 0.2}deg);
          }
          50% {
            transform: translateY(-6px) rotate(${((index % 3) - 1) * 0.2}deg);
          }
        }
      `}</style>

      <button
        className="flex min-h-[120px] w-full flex-col items-center justify-center rounded-2xl border border-gray-100 bg-white p-6 shadow-lg transition-all duration-300 hover:border-blue-200 hover:shadow-xl active:scale-95"
        style={floatingStyle}
        onClick={onClick}
        type="button"
      >
        {/* Icon */}
        <div className={`mb-3 ${iconColor}`}>{icon}</div>

        {/* Title */}
        <span className="text-center leading-tight font-medium text-gray-800">
          {title}
        </span>
      </button>
    </>
  );
}
