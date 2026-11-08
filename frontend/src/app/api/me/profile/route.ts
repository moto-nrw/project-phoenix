import { proxyGet, proxyPut } from "~/lib/route-wrapper.server";
import type {
  BackendProfile,
  ProfileUpdateRequest,
} from "~/lib/profile-helpers";

export const GET = proxyGet("/api/me/profile");

export const PUT = proxyPut<BackendProfile, ProfileUpdateRequest>(
  "/api/me/profile",
);
