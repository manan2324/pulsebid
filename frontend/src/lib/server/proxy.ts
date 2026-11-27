import "server-only";
import { getEnv } from "./env";
import { apiError, mutationGuard, prepareMutation, requestId } from "./http";

const entity = "[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}";
export function allowedCampaign(
  path: string,
  method: string,
  testMode = false,
) {
  if (["advertisers", "bidders", "campaigns"].includes(path))
    return ["GET", "POST"].includes(method);
  if (
    new RegExp(`^(advertisers|bidders|campaigns|creatives)/${entity}$`).test(
      path,
    )
  )
    return ["GET", "PATCH", "DELETE"].includes(method);
  if (path === "snapshot") return method === "GET";
  if (new RegExp(`^campaigns/${entity}/targeting$`).test(path))
    return ["GET", "PUT"].includes(method);
  if (new RegExp(`^campaigns/${entity}/creatives$`).test(path))
    return ["GET", "POST"].includes(method);
  if (new RegExp(`^campaigns/${entity}/(activate|pause)$`).test(path))
    return method === "POST";
  if (new RegExp(`^campaigns/${entity}/budget$`).test(path))
    return method === "GET";
  return (
    testMode &&
    new RegExp(`^campaigns/${entity}/budget/reset$`).test(path) &&
    method === "POST"
  );
}
export function bidderIndex(value: string, count: number) {
  return /^(0|[1-9]\d*)$/.test(value) && Number(value) < count;
}
type ProxyOptions = {
  base: string;
  path: string;
  service: string;
  timeout?: number;
  gateway?: boolean;
  click?: boolean;
};
export async function proxy(request: Request, options: ProxyOptions) {
  const id = requestId(request);
  const guard = mutationGuard(request, id, 0);
  if (guard) return guard;
  const env = getEnv();
  const headers = new Headers({
    Accept: "application/json",
    "X-API-Key": env.PULSEBID_API_KEY,
    "X-Request-Id": id,
  });
  // Only explicitly permitted headers cross the trust boundary.
  for (const name of ["content-type", "if-none-match"]) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (options.gateway && env.TEST_MODE_UI === "true") {
    const value = request.headers.get("x-pulsebid-test-time");
    if (value) headers.set("X-PulseBid-Test-Time", value);
  }
  try {
    const { response: rejection, body: requestBody } = await prepareMutation(
      request,
      id,
    );
    if (rejection) return rejection;
    const response = await fetch(new URL(options.path, options.base), {
      method: request.method,
      headers,
      body: requestBody,
      signal: AbortSignal.timeout(options.timeout ?? 10_000),
      redirect: "manual",
      cache: "no-store",
    });
    const output = new Headers({
      "X-Request-Id": id,
      "Cache-Control": "no-store",
    });
    for (const name of ["content-type", "etag"]) {
      const value = response.headers.get(name);
      if (value) output.set(name, value);
    }
    if (options.gateway)
      for (const name of [
        "server-timing",
        "x-pulsebid-reason",
        "x-pulsebid-debug",
      ]) {
        const value = response.headers.get(name);
        if (value) output.set(`x-upstream-${name}`, value);
      }
    if (options.click && response.status >= 300 && response.status < 400)
      return Response.json(
        { status: response.status, location: response.headers.get("location") },
        { headers: { "X-Request-Id": id, "Cache-Control": "no-store" } },
      );
    // Buffer under the timeout, so slow response bodies also produce the standard envelope.
    const body = [204, 304].includes(response.status)
      ? null
      : await response.arrayBuffer();
    return new Response(body, { status: response.status, headers: output });
  } catch {
    return apiError(503, "unavailable", `${options.service} unreachable`, id);
  }
}
