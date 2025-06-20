"use client";

import { useSession } from "next-auth/react";
import { useEffect, useState } from "react";

interface AuthConfig {
  accessTokenExpiry: string;
  refreshTokenExpiry: string;
  nextAuthSessionLength: string;
  proactiveRefreshWindow: string;
  refreshCooldown: string;
  maxRefreshRetries: number;
  tokenRefreshBehavior: string;
}

export function SessionDebug() {
  const { data: session, status } = useSession();
  const [config, setConfig] = useState<AuthConfig | null>(null);

  useEffect(() => {
    // Fetch auth configuration on mount
    void fetch("/api/auth/config")
      .then(res => res.json() as Promise<{ config: AuthConfig }>)
      .then(data => setConfig(data.config))
      .catch(console.error);
  }, []);

  if (process.env.NODE_ENV === "production") {
    return null; // Don't show in production
  }

  if (status === "loading") {
    return <div className="text-xs text-gray-500">Loading session...</div>;
  }

  return (
    <div className="fixed bottom-4 right-4 bg-gray-900 text-white p-4 rounded-lg shadow-lg text-xs max-w-md">
      <h3 className="font-bold mb-2">Session Debug Info</h3>
      
      {config && (
        <div className="mb-3">
          <h4 className="font-semibold">Configuration:</h4>
          <ul className="ml-2">
            <li>Access Token: {config.accessTokenExpiry}</li>
            <li>Refresh Token: {config.refreshTokenExpiry}</li>
            <li>Session Length: {config.nextAuthSessionLength}</li>
            <li>Refresh Window: {config.proactiveRefreshWindow}</li>
          </ul>
        </div>
      )}
      
      {session ? (
        <div>
          <h4 className="font-semibold">Current Session:</h4>
          <ul className="ml-2">
            <li>User: {session.user?.email}</li>
            <li>Has Token: {session.user?.token ? "Yes" : "No"}</li>
            <li>Session Error: {session.error ?? "None"}</li>
          </ul>
        </div>
      ) : (
        <div>No active session</div>
      )}
    </div>
  );
}