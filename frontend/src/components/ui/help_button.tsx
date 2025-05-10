// components/ui/help-button.tsx
"use client";

import { useState } from "react";
import { Modal } from "./modal";

interface HelpButtonProps {
    title: string;
    content: string | React.ReactNode;
    buttonClassName?: string;
}

export function HelpButton({ title, content, buttonClassName = "" }: HelpButtonProps) {
    const [isOpen, setIsOpen] = useState(false);

    return (
        <>
            <button
                onClick={() => setIsOpen(true)}
                className={`relative inline-flex items-center justify-center rounded-full
                    w-10 h-10 min-w-[40px] min-h-[40px]
                    bg-blue-100 hover:bg-blue-200 
                    text-blue-600 transition-colors duration-200
                    hover:scale-105 transform
                    focus:outline-none focus:ring-2 focus:ring-blue-300 focus:ring-offset-2
                    ${buttonClassName}`}
                title="Hilfe anzeigen"
                aria-label="Hilfe anzeigen"
            >
                <span
                    className="font-bold text-3xl"
                    style={{
                        fontFamily: '"Inter", sans-serif', // TODO: Use other more modern font family here
                        fontSize: '28px',
                        fontWeight: '900',
                        transform: 'translateY(-1px)',
                        letterSpacing: '-0.08em'
                    }}
                >
                    ?
                </span>
            </button>

            <Modal
                isOpen={isOpen}
                onClose={() => setIsOpen(false)}
                title={title}
            >
                <div className="text-left">
                    {/* Content linksbündig mit wichtigen Wörtern bold */}
                    <div className="prose prose-sm max-w-none text-left">
                        {content}
                    </div>

                    {/* Verstanden Button mit Icon am unteren Rand */}
                    <div className="mt-6 flex justify-center">
                        <button
                            onClick={() => setIsOpen(false)}
                            className="inline-flex items-center gap-2 px-4 py-2 text-white rounded-md
                                hover:shadow-lg hover:scale-105 transition-all duration-300
                                shadow-md shadow-green-200 hover:shadow-green-300
                                focus:outline-none focus:ring-2 focus:ring-green-300 focus:ring-offset-2"
                            style={{
                                backgroundColor: '#83cd2d'
                            }}
                        >
                            <span className="font-semibold">Verstanden</span>
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                className="h-4 w-4"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    strokeWidth={2}
                                    d="M5 13l4 4L19 7"
                                />
                            </svg>
                        </button>
                    </div>
                </div>
            </Modal>
        </>
    );
}