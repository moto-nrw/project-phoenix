// components/dashboard/header.tsx (Updated with space-between layout)
"use client";

import Link from "next/link";
import Image from "next/image";
import { Button } from "@/components/ui/button";

interface HeaderProps {
    userName?: string;
}

// Logout Icon als React Component
const LogoutIcon = ({ className }: { className?: string }) => (
    <svg
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={className}
    >
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
        <polyline points="16 17 21 12 16 7" />
        <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
);


export function Header({ userName = "Root" }: HeaderProps) {
    return (
        <header className="w-full bg-white/95 backdrop-blur-md py-4 shadow-sm border-b border-gray-200/50 relative z-50">
            <div className="absolute inset-0 bg-gradient-to-r from-blue-50/80 via-white/40 to-green-50/80 md:bg-gradient-to-r md:from-blue-50/60 md:via-white/30 md:to-green-50/60"></div>
            <div className="relative w-full px-4 flex items-center justify-between">
                {/* Left container: Logo, MOTO text, and welcome message */}
                <div className="flex items-center gap-3">
                    <Image
                        src="/images/moto_transparent.png"
                        alt="Logo"
                        width={40}
                        height={40}
                        className="h-8 md:h-10 w-auto"
                    />
                    <span
                        className="hidden md:inline text-2xl md:text-3xl font-extrabold"
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
                        moto
                    </span>
                    <h1 className="text-base md:text-lg lg:text-2xl font-bold ml-2 md:ml-3 lg:ml-6">
                        <span className="hidden md:inline">Willkommen, {userName}!</span>
                        <span className="md:hidden">Hallo, {userName.split(' ')[0]}</span>
                    </h1>
                </div>

                {/* Right container: Logout button */}
                <div className="flex-shrink-0">
                    <Link href="/logout">
                        <Button
                            variant="outline_danger"
                            size="sm"
                            className="group relative flex items-center gap-2 overflow-hidden border-gray-300 hover:border-red-400 hover:bg-red-50/80 text-[#FF3130] hover:text-[#FF3130] transition-all duration-200"
                        >
                            {/* Icon mit Animation */}
                            <LogoutIcon className="w-4 h-4 text-[#FF3130] group-hover:text-red-600 transition-colors duration-200" />

                            {/* Text nur auf Desktop sichtbar */}
                            <span className="hidden sm:inline group-hover:text-[#FF3130] transition-colors duration-200">
                                Abmelden
                            </span>

                            {/* Subtiler Hover-Effekt */}
                            <div className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
                                <div className="absolute inset-0 bg-gradient-to-r from-red-50/0 via-red-50/40 to-red-50/0"></div>
                            </div>
                        </Button>
                    </Link>
                </div>
            </div>
        </header>
    );
}