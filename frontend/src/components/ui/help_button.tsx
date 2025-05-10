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
                className={`inline-flex items-center justify-center rounded-full bg-blue-100 hover:bg-blue-200 
          text-blue-600 transition-all duration-200 hover:scale-110 p-2 ${buttonClassName}`}
                title="Hilfe anzeigen"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-5 w-5"
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
                title="" // Pass empty title to Modal
            >
                <div className="text-left">
                    {/* Keep your custom title */}
                    <h2 className="text-xl font-bold text-gray-900 mb-4">
                        {title}
                    </h2>

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
                                shadow-md shadow-green-200 hover:shadow-green-300"
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