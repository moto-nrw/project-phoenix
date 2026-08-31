import { proxyGet, proxyPost } from "~/lib/route-proxy.server";
import type {
  BackendGroupHandover,
  BackendAdditionalSupervisionResult,
  BackendSubstitutionOverview,
  AddSupervisorRequest,
  CreateSubstitutionRequest,
} from "~/lib/substitution-helpers";

export const GET = proxyGet<BackendSubstitutionOverview>("/api/substitutions");

export const POST = proxyPost<
  BackendGroupHandover | BackendAdditionalSupervisionResult,
  CreateSubstitutionRequest | AddSupervisorRequest
>("/api/substitutions");
