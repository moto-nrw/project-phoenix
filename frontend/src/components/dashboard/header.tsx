// components/dashboard/header.tsx (Final Version mit Gradient)
"use client";

import Link from "next/link";
import Image from "next/image";
import { Button } from "@/components/ui/button";

interface HeaderProps {
  userName?: string;
}

export function Header({ userName = "Root" }: HeaderProps) {
  return (
      <header className="w-full bg-white/80 p-4 shadow-sm backdrop-blur-sm">
        <div className="container mx-auto flex items-center justify-between">
          <div className="flex items-center">
            {/* MOTO Logo-Text und Logo kombiniert */}
            <div className="flex items-center gap-3">
              <Image
                  src="/images/moto_transparent.png"
                  alt="Logo"
                  width={40}
                  height={40}
                  className="h-10 w-auto"
              />
              {/* Using Tailwind classes with custom styles */}
              <span
                  className="text-2xl font-extrabold inline-block"
                  style={{
                    fontFamily: 'var(--font-geist-sans)',
                    letterSpacing: '-0.5px',
                    fontWeight: 800,
                    background: 'linear-gradient(135deg, #5080d8, #83cd2d)',
                    WebkitBackgroundClip: 'text',
                    backgroundClip: 'text',
                    WebkitTextFillColor: 'transparent',
                  }}
              >
              MOTO
            </span>
            </div>

            {/* Title section */}
            <div className="flex items-center ml-6">
              <h1 className="text-xl font-bold">
                <span className="hidden md:inline">Willkommen, {userName}!</span>
              </h1>
            </div>
          </div>

          {/* Logout button on the right */}
          <div>
            <Link href="/logout">
              <Button>Abmelden</Button>
            </Link>
          </div>
        </div>
      </header>
  );
}