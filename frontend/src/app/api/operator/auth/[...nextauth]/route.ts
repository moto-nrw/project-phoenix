import { operatorHandlers } from "~/server/auth/operator";
import { withPrivateNoStore } from "~/lib/private-no-store";

export const GET = withPrivateNoStore(operatorHandlers.GET);
export const POST = withPrivateNoStore(operatorHandlers.POST);
