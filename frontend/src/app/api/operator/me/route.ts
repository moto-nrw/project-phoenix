import { NextResponse } from "next/server";
import { getOperatorToken } from "~/lib/operator/cookies";
import { operatorApiGet } from "~/lib/operator/route-wrapper";

interface ProfileResponse {
  id: number;
  email: string;
  display_name: string;
}

export async function GET() {
  const token = await getOperatorToken();
  if (!token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const profile = await operatorApiGet<ProfileResponse>(
      "/operator/profile",
      token,
    );

    return NextResponse.json({
      id: String(profile.id),
      email: profile.email,
      displayName: profile.display_name,
    });
  } catch {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }
}
