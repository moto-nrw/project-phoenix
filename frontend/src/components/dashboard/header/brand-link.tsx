// Brand link component for header
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import Link from "next/link";
import Image from "next/image";

/**
 * Brand link with logo and text
 */
interface BrandLinkProps {
  readonly isScrolled?: boolean;
}

export function BrandLink({ isScrolled = false }: BrandLinkProps) {
  return (
    <Link href="/dashboard" className="group flex items-center space-x-3">
      <div className="relative transition-transform duration-200 group-hover:scale-110">
        <Image
          src="/images/moto_transparent.png"
          alt="moto"
          width={40}
          height={40}
          className="h-9 w-9"
          priority
        />
        {/* Subtle glow effect */}
        <div className="absolute inset-0 -z-10 h-9 w-9 rounded-full bg-gradient-to-br from-[#5080d8]/20 to-[#83cd2d]/20 blur-sm" />
      </div>

      <div className="flex items-center space-x-3">
        <span
          className={`font-bold tracking-tight transition-all duration-300 group-hover:scale-105 ${
            isScrolled ? "text-lg lg:text-xl" : "text-xl"
          }`}
          style={{
            background: "linear-gradient(135deg, #5080d8, #83cd2d)",
            WebkitBackgroundClip: "text",
            backgroundClip: "text",
            WebkitTextFillColor: "transparent",
          }}
        >
          moto
        </span>
      </div>
    </Link>
  );
}

/**
 * Vertical separator for breadcrumb area
 */
export function BreadcrumbDivider() {
  return <div className="hidden h-5 w-px bg-gray-300 md:block" />;
}
