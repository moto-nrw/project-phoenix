// components/ui/help-button.tsx
"use client";

import { useState } from "react";
import { usePathname } from "next/navigation";
import { Modal } from "./modal";

interface HelpButtonProps {
  readonly title: string;
  readonly content: string | React.ReactNode;
  readonly buttonClassName?: string;
}

export function HelpButton({
  title,
  content,
  buttonClassName = "",
}: HelpButtonProps) {
  const [isOpen, setIsOpen] = useState(false);
  const pathname = usePathname();
  const isLoginPage = pathname === "/";

  return (
    <>
      <button
        onClick={() => setIsOpen(true)}
        className={`relative inline-flex h-10 min-h-[40px] w-10 min-w-[40px] items-center justify-center rounded-full bg-gray-100/40 text-gray-600 transition-colors duration-200 hover:bg-gray-200/60 hover:text-gray-700 focus:ring-2 focus:ring-gray-300 focus:ring-offset-2 focus:outline-none ${buttonClassName}`}
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

      <Modal isOpen={isOpen} onClose={() => setIsOpen(false)} title={title}>
        {/* Modern content styling */}
        <div className="prose prose-gray max-w-none">{content}</div>

        {/* Impressum Link - nur auf der Login-Seite anzeigen */}
        {isLoginPage && (
          <div className="mt-6 border-t border-gray-200 pt-4">
            <a
              href="https://moto.nrw/impressum/"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 text-sm text-gray-500 transition-colors duration-200 hover:text-gray-700 hover:underline"
              aria-label="Zum Impressum"
            >
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-1M10 6V4a2 2 0 012-2h8a2 2 0 012 2v8a2 2 0 01-2 2h-2M10 6l8 8"
                />
              </svg>
              Impressum
            </a>
          </div>
        )}
      </Modal>
    </>
  );
}
