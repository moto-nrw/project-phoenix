import { proxyGet, proxyPut } from "~/lib/operator/route-wrapper.server";

interface ProfileResponse {
  id: number;
  email: string;
  display_name: string;
}

interface UpdateProfileBody {
  display_name: string;
}

export const GET = proxyGet<ProfileResponse>("/operator/profile");

export const PUT = proxyPut<ProfileResponse, UpdateProfileBody>(
  "/operator/profile",
);
