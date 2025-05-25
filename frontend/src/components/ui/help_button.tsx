// components/ui/help-button.tsx
"use client";

import { useState } from "react";
import { usePathname } from "next/navigation";
import { Modal } from "./modal";

interface HelpButtonProps {
    title: string;
    content: string | React.ReactNode;
    buttonClassName?: string;
}

export function HelpButton({ title, content, buttonClassName = "" }: HelpButtonProps) {
    const [isOpen, setIsOpen] = useState(false);
    const pathname = usePathname();
    const isLoginPage = pathname === "/";

    return (
        <>
            <button
                onClick={() => setIsOpen(true)}
                className={`relative inline-flex items-center justify-center
                    w-10 h-10 min-w-[40px] min-h-[40px]
                    bg-blue-100/40 hover:bg-blue-200/60
                    text-blue-600 hover:text-blue-700 
                    transition-colors duration-200
                    rounded-full
                    focus:outline-none focus:ring-2 focus:ring-blue-300 focus:ring-offset-2
                    ${buttonClassName}`}
                title="Hilfe anzeigen"
                aria-label="Hilfe anzeigen"
            >
                <svg
                    className="h-6 w-6"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                >
                    <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                </svg>
            </button>

            <Modal
                isOpen={isOpen}
                onClose={() => setIsOpen(false)}
                title={title}
            >
                {/* Modern content styling */}
                <div className="prose prose-gray max-w-none">
                    {content}
                </div>

                {/* Impressum Link - nur auf der Login-Seite anzeigen */}
                {isLoginPage && (
                    <div className="mt-6 pt-4 border-t border-gray-200">
                        <a
                            href="https://moto.nrw/impressum/"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-2 text-sm text-gray-500 hover:text-gray-700 transition-colors duration-200 hover:underline"
                            aria-label="Zum Impressum"
                        >
                            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-1M10 6V4a2 2 0 012-2h8a2 2 0 012 2v8a2 2 0 01-2 2h-2M10 6l8 8" />
                            </svg>
                            Impressum
                        </a>
                    </div>
                )}
            </Modal>
        </>
    );
}