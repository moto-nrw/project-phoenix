"use client";

import React, { useState } from "react";
import { cn } from "~/lib/utils";

interface GlassButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "default" | "neutral" | "back";
  children: React.ReactNode;
}

export function GlassButton({
  children,
  variant = "default",
  className,
  ...props
}: GlassButtonProps) {
  const [isHovering, setIsHovering] = useState(false);
  const [isPressed, setIsPressed] = useState(false);

  const baseStyles = cn(
    "relative inline-flex items-center justify-center px-8 py-3",
    "text-lg font-medium transition-all duration-300",
    "backdrop-blur-xl bg-white/10 border border-white/20",
    "shadow-lg hover:shadow-xl",
    variant !== "back" && "rounded-full",
    variant === "back" && "rounded-lg",
    isHovering && variant !== "back" && "scale-[1.02] -translate-y-[2px]",
    isPressed && "scale-[0.98]"
  );

  const variantStyles = {
    default: cn(
      "text-black bg-gradient-to-r from-white/90 to-white/80",
      "hover:from-white/95 hover:to-white/90",
      "border-gray-200/50"
    ),
    neutral: cn(
      "text-gray-700 bg-white/50",
      "hover:bg-white/60",
      "border-gray-300/30"
    ),
    back: cn(
      "text-gray-600 bg-white/40",
      "hover:bg-white/50",
      "border-gray-200/20",
      "text-base px-6 py-2"
    )
  };

  return (
    <div className="relative inline-block">
      {/* Gradient glow for default variant */}
      {variant === "default" && (
        <div
          className={cn(
            "absolute inset-0 rounded-full opacity-0 blur-xl transition-opacity duration-300",
            isHovering && "opacity-20"
          )}
          style={{
            background: "linear-gradient(90deg, #5080d8 0%, #83cd2d 100%)",
          }}
        />
      )}
      
      <button
        className={cn(baseStyles, variantStyles[variant], className)}
        onMouseEnter={() => setIsHovering(true)}
        onMouseLeave={() => {
          setIsHovering(false);
          setIsPressed(false);
        }}
        onMouseDown={() => setIsPressed(true)}
        onMouseUp={() => setIsPressed(false)}
        {...props}
      >
        {/* Glass shine effect */}
        <div className={cn(
          "absolute inset-0 overflow-hidden pointer-events-none",
          variant !== "back" ? "rounded-full" : "rounded-lg"
        )}>
          <div
            className={cn(
              "absolute inset-0 bg-gradient-to-tr from-transparent via-white/10 to-transparent",
              "transform -translate-x-full transition-transform duration-700",
              isHovering && "translate-x-full"
            )}
            style={{
              transform: isHovering ? "translateX(100%)" : "translateX(-100%)",
            }}
          />
        </div>
        
        {/* Content */}
        <span className="relative z-10">
          {children}
        </span>
      </button>
    </div>
  );
}