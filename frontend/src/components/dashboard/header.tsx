// components/dashboard/header.tsx (Aktualisierte Version mit inline SVG)
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

                {/* Moderner Logout button */}
                <div>
                    <Link href="/logout">
                        <Button
                            variant="outline_danger"
                            size="sm"
                            className="group relative flex items-center gap-2 overflow-hidden border-gray-300 hover:border-red-400 hover:bg-red-50/80 text-red-700 hover:text-red-700 transition-all duration-200"
                        >
                            {/* Icon mit Animation */}
                            <LogoutIcon className="w-4 h-4 text-red-700 group-hover:text-red-600 transition-colors duration-200" />

                            {/* Text nur auf Desktop sichtbar */}
                            <span className="hidden sm:inline group-hover:text-red-700 transition-colors duration-200">
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