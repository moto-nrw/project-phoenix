import { proxyGet, proxyPost } from "~/lib/route-proxy.server";
import type {
  BackendGroupHandover,
  BackendSubstitutionOverview,
  CreateSubstitutionRequest,
} from "~/lib/substitution-helpers";

export const GET = proxyGet<BackendSubstitutionOverview>("/api/substitutions");

export const POST = proxyPost<BackendGroupHandover, CreateSubstitutionRequest>(
  "/api/substitutions",
);
