import { NextResponse } from "next/server";
import { clearOperatorTokens } from "~/lib/operator/cookies";

export async function POST() {
  await clearOperatorTokens();
  return NextResponse.json({ success: true });
}
