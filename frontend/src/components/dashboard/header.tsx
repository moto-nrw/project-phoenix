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
          {/* Logo and title - always in the same position */}
          <div className="flex w-[160px] items-center justify-center gap-3">
            <Image
              src="/images/moto_transparent.png"
              alt="Logo"
              width={40}
              height={40}
              className="h-10 w-auto"
            />
          </div>

          {/* Title section */}
          <div className="flex items-center">
            <h1 className="text-xl font-bold">
              <span className="hidden md:inline">Willkommen, {userName}!</span>
            </h1>
          </div>
        </div>

        {/* Logout button on the right */}
        <div>
          <Link href="/logout">
            <Button>Logout</Button>
          </Link>
        </div>
      </div>
    </header>
  );
}
