// Profile dropdown component for header
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import Link from "next/link";
import Image from "next/image";

/**
 * User avatar with initials fallback
 */
interface UserAvatarProps {
  readonly avatarUrl?: string | null;
  readonly userName: string;
  readonly size?: "sm" | "md";
}

export function UserAvatar({
  avatarUrl,
  userName,
  size = "sm",
}: UserAvatarProps) {
  const sizeClasses = size === "sm" ? "w-8 h-8 text-sm" : "w-11 h-11 text-base";
  const initials = getInitials(userName);

  return (
    <div
      className={`relative ${sizeClasses} flex flex-shrink-0 items-center justify-center overflow-hidden rounded-full font-semibold text-white ${size === "sm" ? "shadow-sm ring-2 ring-white" : "shadow-md"}`}
      style={{
        background: avatarUrl
          ? "transparent"
          : "linear-gradient(135deg, #5080d8, #83cd2d)",
      }}
    >
      {avatarUrl ? (
        <Image
          src={avatarUrl}
          alt={userName}
          fill
          className="object-cover"
          sizes="44px"
          unoptimized
        />
      ) : (
        initials
      )}
    </div>
  );
}

function getInitials(userName: string): string {
  return (
    (userName?.trim() || "")
      .split(" ")
      .filter((n) => n.length > 0)
      .map((n) => n[0])
      .join("")
      .toUpperCase() || "?"
  );
}

/**
 * Logout icon SVG
 */
export function LogoutIcon({ className }: Readonly<{ className?: string }>) {
  return (
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
}

/**
 * Chevron down icon for dropdown toggle
 */
interface ChevronDownIconProps {
  readonly isOpen: boolean;
}

export function ChevronDownIcon({ isOpen }: ChevronDownIconProps) {
  return (
    <svg
      className={`h-4 w-4 text-gray-400 transition-all duration-200 ${isOpen ? "rotate-180 text-gray-600" : ""}`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

/**
 * Settings icon
 */
function SettingsIcon() {
  return (
    <svg
      className="mr-3 h-4 w-4 text-gray-400 transition-colors group-hover:text-gray-600 group-active:text-white"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M10.325 4.317c.426-1.756 2.924-1.756 3.50 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
      />
    </svg>
  );
}

/**
 * Help icon
 */
function HelpIcon() {
  return (
    <svg
      className="mr-3 h-4 w-4 text-gray-400 transition-colors group-hover:text-gray-600 group-active:text-white"
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
  );
}

/**
 * Profile dropdown trigger button
 */
interface ProfileTriggerProps {
  readonly displayName: string;
  readonly displayAvatar?: string | null;
  readonly userRole: string;
  readonly isOpen: boolean;
  readonly onClick: () => void;
}

export function ProfileTrigger({
  displayName,
  displayAvatar,
  userRole,
  isOpen,
  onClick,
}: ProfileTriggerProps) {
  return (
    <button
      onClick={onClick}
      className="flex touch-manipulation items-center space-x-2 rounded-lg p-1.5 transition-colors duration-200 hover:bg-gray-100 active:bg-gray-200"
    >
      <UserAvatar avatarUrl={displayAvatar} userName={displayName} size="sm" />

      <div className="hidden text-left md:block">
        <div className="text-sm font-medium text-gray-900">{displayName}</div>
        <div className="text-xs text-gray-500">{userRole}</div>
      </div>

      <ChevronDownIcon isOpen={isOpen} />
    </button>
  );
}

/**
 * Profile dropdown menu
 */
interface ProfileDropdownMenuProps {
  readonly isOpen: boolean;
  readonly displayName: string;
  readonly displayAvatar?: string | null;
  readonly userEmail: string;
  readonly onClose: () => void;
  readonly onLogout: () => void;
}

export function ProfileDropdownMenu({
  isOpen,
  displayName,
  displayAvatar,
  userEmail,
  onClose,
  onLogout,
}: ProfileDropdownMenuProps) {
  const handleHelpClick = (e: React.MouseEvent) => {
    e.preventDefault();
    onClose();
    // Trigger help button click after a small delay to ensure menu closes first
    setTimeout(() => {
      const helpButton = document.querySelector(
        '[aria-label="Hilfe anzeigen"]',
      );
      if (helpButton) {
        (helpButton as HTMLButtonElement).click();
      }
    }, 100);
  };

  const handleLogoutClick = (e: React.MouseEvent) => {
    e.preventDefault();
    onClose();
    onLogout();
  };

  return (
    <>
      {/* Backdrop for mobile - native button handles Enter/Space automatically */}
      {isOpen && (
        <button
          type="button"
          className="fixed inset-0 z-40 cursor-default md:hidden"
          onClick={onClose}
          aria-label="Menü schließen"
        />
      )}

      {/* Dropdown menu */}
      <div
        className={`absolute top-full right-0 z-50 mt-2 w-72 rounded-2xl border border-gray-200/50 bg-white/95 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-300 ease-out ${
          isOpen
            ? "visible translate-y-0 opacity-100"
            : "invisible -translate-y-2 opacity-0"
        }`}
      >
        {/* User info header */}
        <div className="border-b border-gray-100/50 px-4 py-4">
          <div className="flex items-center space-x-3">
            <UserAvatar
              avatarUrl={displayAvatar}
              userName={displayName}
              size="md"
            />
            <div className="min-w-0 flex-1">
              <div className="truncate font-semibold text-gray-900">
                {displayName}
              </div>
              <div className="truncate text-xs text-gray-500" title={userEmail}>
                {userEmail}
              </div>
            </div>
          </div>
        </div>

        {/* Menu items */}
        <div className="p-2">
          <Link
            href="/settings"
            onClick={onClose}
            className="group flex items-center rounded-xl px-3 py-2.5 text-sm font-medium text-gray-700 transition-all duration-200 ease-out hover:bg-gray-100 hover:text-gray-900 active:bg-gray-900 active:text-white"
          >
            <SettingsIcon />
            Einstellungen
          </Link>

          <button
            onClick={handleHelpClick}
            className="group flex w-full items-center rounded-xl px-3 py-2.5 text-left text-sm font-medium text-gray-700 transition-all duration-200 ease-out hover:bg-gray-100 hover:text-gray-900 active:bg-gray-900 active:text-white"
          >
            <HelpIcon />
            Hilfe & Support
          </button>

          {/* Divider */}
          <div className="my-2 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent" />

          {/* Logout button */}
          <button
            onClick={handleLogoutClick}
            className="group flex w-full items-center rounded-xl px-3 py-2.5 text-left text-sm font-medium text-red-600 transition-all duration-200 ease-out hover:bg-red-50 hover:text-red-700 active:bg-red-600 active:text-white"
          >
            <LogoutIcon className="mr-3 h-4 w-4 transition-colors group-active:text-white" />
            Abmelden
          </button>
        </div>
      </div>
    </>
  );
}
