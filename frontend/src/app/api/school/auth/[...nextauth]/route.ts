import { schoolHandlers } from "~/server/auth/school";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const GET = withPrivateNoStore(schoolHandlers.GET);
export const POST = withPrivateNoStore(schoolHandlers.POST);
