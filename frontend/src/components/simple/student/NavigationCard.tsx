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
  index = 0 
}: NavigationCardProps) {
  // Floating animation style from ogs_groups pattern
  const floatingStyle = {
    animation: `float 8s ease-in-out infinite ${index * 0.5}s`,
    transform: `rotate(${(index % 3 - 1) * 0.2}deg)`,
  };

  return (
    <>
      <style jsx>{`
        @keyframes float {
          0%, 100% { transform: translateY(0px) rotate(${(index % 3 - 1) * 0.2}deg); }
          50% { transform: translateY(-6px) rotate(${(index % 3 - 1) * 0.2}deg); }
        }
      `}</style>
      
      <button
        className="flex flex-col items-center justify-center rounded-2xl bg-white p-6 shadow-lg hover:shadow-xl transition-all duration-300 border border-gray-100 hover:border-blue-200 active:scale-95 w-full min-h-[120px]"
        style={floatingStyle}
        onClick={onClick}
        type="button"
      >
        {/* Icon */}
        <div className={`mb-3 ${iconColor}`}>
          {icon}
        </div>
        
        {/* Title */}
        <span className="text-gray-800 font-medium text-center leading-tight">
          {title}
        </span>
      </button>
    </>
  );
}