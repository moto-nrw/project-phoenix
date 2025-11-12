"use client";

import React, { useRef, useState } from "react";
import { cn } from "~/lib/utils";

interface HoverBorderGradientProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  containerClassName?: string;
  className?: string;
  children: React.ReactNode;
  as?: React.ElementType;
  duration?: number;
}

export function HoverBorderGradient({
  children,
  containerClassName,
  className,
  as: Component = "button",
  duration: _duration = 1,
  ...props
}: HoverBorderGradientProps) {
  const [isHovering, setIsHovering] = useState(false);
  const [isPressed, setIsPressed] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  return (
    <div
      ref={ref}
      className={cn(
        "group relative inline-block rounded-full p-[2px] transition-all duration-300",
        isHovering && "-translate-y-[3px] scale-[1.02]",
        isPressed && "scale-[0.98]",
        containerClassName,
      )}
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => {
        setIsHovering(false);
        setIsPressed(false);
      }}
      onMouseDown={() => setIsPressed(true)}
      onMouseUp={() => setIsPressed(false)}
    >
      {/* Gradient border background */}
      <div
        className={cn(
          "absolute inset-0 rounded-full opacity-100 transition-all duration-300",
          isHovering && "opacity-100 brightness-110",
        )}
        style={{
          background: "linear-gradient(90deg, #5080d8 0%, #83cd2d 100%)",
        }}
      />

      {/* Rotating light effect */}
      <div
        className={cn(
          "absolute inset-0 rounded-full opacity-0 transition-opacity duration-300",
          isHovering && "opacity-100",
        )}
      >
        <div
          className="animate-spin-slow absolute inset-0 rounded-full"
          style={{
            background: `conic-gradient(from 0deg, transparent 0deg, rgba(255,255,255,0.3) 90deg, transparent 180deg)`,
          }}
        />
      </div>

      {/* Glow effect */}
      <div
        className={cn(
          "absolute -inset-4 rounded-full opacity-0 blur-xl transition-opacity duration-300",
          isHovering && "opacity-10",
        )}
        style={{
          background: "linear-gradient(90deg, #5080d8 0%, #83cd2d 100%)",
        }}
      />

      {/* Button content */}
      <Component
        className={cn(
          "relative z-10 flex h-full w-full items-center justify-center rounded-full bg-white text-black transition-all duration-300",
          isHovering && "brightness-110",
          className,
        )}
        {...props}
      >
        {children}
      </Component>

      {/* Double shadow for depth */}
      <div
        className={cn(
          "absolute inset-0 -z-10 rounded-full transition-all duration-300",
          "shadow-md",
          isHovering && "shadow-lg",
        )}
        style={{
          boxShadow: isHovering
            ? "0 8px 25px -8px rgba(80, 128, 216, 0.3), 0 6px 15px -4px rgba(131, 205, 45, 0.2)"
            : "0 4px 15px -4px rgba(80, 128, 216, 0.2), 0 2px 10px -2px rgba(131, 205, 45, 0.15)",
        }}
      />
    </div>
  );
}
