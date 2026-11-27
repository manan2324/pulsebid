import { getEnv } from "@/lib/server/env";
import { allowedCampaign, proxy } from "@/lib/server/proxy";
import { apiError, requestId } from "@/lib/server/http";

async function handle(
  request: Request,
  context: { params: Promise<{ path: string[] }> },
) {
  const { path } = await context.params;
  const joined = path.join("/");
  const env = getEnv();
  if (!allowedCampaign(joined, request.method, env.TEST_MODE_UI === "true"))
    return apiError(
      404,
      "not_found",
      "Unknown route or method",
      requestId(request),
    );
  return proxy(request, {
    base: env.CAMPAIGN_API_URL,
    path: `/v1/${joined}${new URL(request.url).search}`,
    service: "Campaign service",
  });
}

export {
  handle as GET,
  handle as POST,
  handle as PUT,
  handle as PATCH,
  handle as DELETE,
  handle as HEAD,
  handle as OPTIONS,
};
