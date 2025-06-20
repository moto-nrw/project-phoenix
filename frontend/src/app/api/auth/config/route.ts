import { NextResponse } from "next/server";

/**
 * Endpoint to check the current authentication configuration
 * Useful for debugging token expiry settings
 */
export async function GET() {
  const config = {
    accessTokenExpiry: "15 minutes",
    refreshTokenExpiry: "24 hours",
    nextAuthSessionLength: "24 hours",
    proactiveRefreshWindow: "10 minutes before expiry",
    refreshCooldown: "30 seconds between attempts",
    maxRefreshRetries: 3,
    tokenRefreshBehavior: "Tokens refresh automatically when access token has less than 10 minutes remaining"
  };

  console.log("=== Current Auth Configuration ===");
  console.log(JSON.stringify(config, null, 2));
  console.log("=================================");

  return NextResponse.json({
    success: true,
    config
  });
}