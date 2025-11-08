"use client";

import React, { useState } from "react";
import { cn } from "~/lib/utils";

interface GradientBorderButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
}

export function GradientBorderButton({
  children,
  className,
  ...props
}: GradientBorderButtonProps) {
  const [isHovering, setIsHovering] = useState(false);
  const [isPressed, setIsPressed] = useState(false);

  return (
    <div className="relative inline-block">
      {/* Gradient border container */}
      <div
        className={cn(
          "relative rounded-full p-[2px]",
          "bg-gradient-to-r from-[#5080d8] to-[#83cd2d]",
          "transition-all duration-300",
          isHovering && "-translate-y-[2px] scale-[1.02]",
          isPressed && "scale-[0.98]",
        )}
      >
        <button
          className={cn(
            "relative rounded-full px-10 py-3",
            "bg-white text-lg font-medium text-black",
            "transition-all duration-300",
            "hover:bg-gray-50",
            className,
          )}
          onMouseEnter={() => setIsHovering(true)}
          onMouseLeave={() => {
            setIsHovering(false);
            setIsPressed(false);
          }}
          onMouseDown={() => setIsPressed(true)}
          onMouseUp={() => setIsPressed(false)}
          {...props}
        >
          {children}
        </button>
      </div>

      {/* Subtle glow effect */}
      <div
        className={cn(
          "absolute inset-0 -z-10 rounded-full blur-lg transition-opacity duration-300",
          isHovering ? "opacity-30" : "opacity-0",
        )}
        style={{
          background: "linear-gradient(90deg, #5080d8 0%, #83cd2d 100%)",
        }}
      />
    </div>
  );
}
