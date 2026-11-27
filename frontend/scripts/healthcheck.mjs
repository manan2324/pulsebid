import { createHmac } from "node:crypto";
try {
  const secret = process.env.SESSION_SECRET;
  if (!secret || secret.length < 32)
    throw new Error("Invalid session configuration");
  const now = Math.floor(Date.now() / 1000);
  const encode = (value) =>
    Buffer.from(JSON.stringify(value)).toString("base64url");
  const signed = `${encode({ alg: "HS256" })}.${encode({ role: "operator", sub: "operator", iss: "pulsebid-console", aud: "pulsebid-console", iat: now, exp: now + 30 })}`;
  const token = `${signed}.${createHmac("sha256", secret).update(signed).digest("base64url")}`;
  const response = await fetch(
    `http://127.0.0.1:${process.env.PORT ?? 3001}/api/health`,
    {
      headers: { Cookie: `pb_session=${token}` },
      signal: AbortSignal.timeout(4000),
    },
  );
  const data = await response.json();
  if (
    !response.ok ||
    !Array.isArray(data.services) ||
    typeof data.checked_at !== "string"
  )
    throw new Error("Console health probe failed");
} catch {
  process.exitCode = 1;
}
