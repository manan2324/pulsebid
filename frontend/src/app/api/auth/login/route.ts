import { createHash, timingSafeEqual } from "node:crypto";
import { NextRequest, NextResponse } from "next/server";
import { z } from "zod";
import { getEnv } from "@/lib/server/env";
import { allowLogin } from "@/lib/server/rateLimit";
import {
  createSession,
  SESSION_COOKIE,
  SESSION_SECONDS,
} from "@/lib/server/session";
import {
  apiError,
  mutationGuard,
  prepareMutation,
  requestId,
} from "@/lib/server/http";

export async function POST(request: NextRequest) {
  const id = requestId(request);
  const guard = mutationGuard(request, id, 0);
  if (guard) return guard;
  // A deployment proxy must overwrite X-Real-IP. Without a trusted proxy all callers share one bucket.
  const ip =
    process.env.TRUST_PROXY === "true"
      ? (request.headers.get("x-real-ip") ?? "shared")
      : "shared";
  if (!allowLogin(ip))
    return apiError(
      429,
      "rate_limited",
      "Too many attempts. Try again in one minute.",
      id,
    );
  const prepared = await prepareMutation(request, id);
  if (prepared.response) return prepared.response;
  let payload: unknown;
  try {
    payload = JSON.parse(new TextDecoder().decode(prepared.body));
  } catch {
    payload = null;
  }
  const parsed = z
    .object({ password: z.string().max(1024) })
    .safeParse(payload);
  if (!parsed.success)
    return apiError(400, "invalid_request", "Enter a password", id);
  const env = getEnv();
  const hash = (value: string) => createHash("sha256").update(value).digest();
  if (!timingSafeEqual(hash(parsed.data.password), hash(env.ADMIN_PASSWORD)))
    return apiError(401, "unauthorized", "Incorrect password", id);
  const response = NextResponse.json(
    { ok: true },
    { headers: { "X-Request-Id": id, "Cache-Control": "no-store" } },
  );
  response.cookies.set(
    SESSION_COOKIE,
    await createSession(env.SESSION_SECRET),
    {
      httpOnly: true,
      sameSite: "lax",
      secure: env.NODE_ENV === "production",
      path: "/",
      maxAge: SESSION_SECONDS,
    },
  );
  return response;
}
