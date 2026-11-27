import { getEnv } from "@/lib/server/env";
import { proxy } from "@/lib/server/proxy";
import { prometheusPath } from "@/lib/server/promQueries";
import { apiError, requestId } from "@/lib/server/http";
export async function GET(request: Request) {
  const path = prometheusPath(new URL(request.url).searchParams);
  if (!path)
    return apiError(
      400,
      "invalid_request",
      "Choose a named query and a supported range",
      requestId(request),
    );
  return proxy(request, {
    base: getEnv().PROMETHEUS_URL,
    path,
    service: "Prometheus",
  });
}
