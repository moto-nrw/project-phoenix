"use client";

import React from "react";
import Link from "next/link";

interface SettingsCardProps {
  title: string;
  description: string;
  icon: React.ReactNode;
  href?: string;
  onClick?: () => void;
  isActive?: boolean;
  badge?: string;
  index?: number;
  children?: React.ReactNode;
}

export function SettingsCard({
  title,
  description,
  icon,
  href,
  onClick,
  isActive = false,
  badge,
  index = 0,
  children,
}: SettingsCardProps) {
  // Floating animation style from ogs_groups
  const floatingStyle = {
    animation: `float 8s ease-in-out infinite ${index * 0.7}s`,
    transform: `rotate(${(index % 3 - 1) * 0.5}deg)`,
  };

  const cardContent = (
    <>
      <div className="flex items-start gap-4">
        <div
          className={`flex-shrink-0 p-3 rounded-2xl transition-colors ${
            isActive
              ? "bg-gradient-to-br from-[#5080D8] to-[#4070C8] text-white"
              : "bg-gray-100 text-gray-600"
          }`}
        >
          {icon}
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-gray-900 text-lg mb-1">
            {title}
            {badge && (
              <span className="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-900 text-white">
                {badge}
              </span>
            )}
          </h3>
          <p className="text-sm text-gray-600 leading-relaxed">{description}</p>
        </div>
      </div>
      {children && <div className="mt-4">{children}</div>}
    </>
  );

  const cardClasses = `
    block w-full bg-white rounded-3xl shadow-md p-6
    transition-all duration-300 hover:shadow-lg hover:scale-[1.02] hover:-translate-y-1
    active:scale-[0.98] active:shadow-md
    ${isActive ? "ring-2 ring-[#5080D8] ring-opacity-50" : ""}
  `;

  // Use Link for external navigation, button for actions
  if (href?.startsWith("/")) {
    return (
      <Link href={href} className={cardClasses} style={floatingStyle}>
        {cardContent}
      </Link>
    );
  }

  return (
    <button
      onClick={onClick}
      className={cardClasses}
      style={floatingStyle}
      type="button"
    >
      {cardContent}
    </button>
  );
}

// CSS for floating animation (add to global styles or component)
export const floatingAnimation = `
  @keyframes float {
    0%, 100% {
      transform: translateY(0px) rotate(var(--rotation));
    }
    50% {
      transform: translateY(-4px) rotate(var(--rotation));
    }
  }
`;