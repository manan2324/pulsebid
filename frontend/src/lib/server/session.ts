import { SignJWT } from "jose/jwt/sign";
import { jwtVerify } from "jose/jwt/verify";

export const SESSION_COOKIE = "pb_session";
export const SESSION_SECONDS = 12 * 60 * 60;
const issuer = "pulsebid-console";
function key(secret: string) {
  if (secret.length < 32)
    throw new Error("SESSION_SECRET must have at least 32 characters");
  return new TextEncoder().encode(secret);
}
export async function createSession(secret: string) {
  return new SignJWT({ role: "operator" })
    .setProtectedHeader({ alg: "HS256" })
    .setSubject("operator")
    .setIssuer(issuer)
    .setAudience(issuer)
    .setIssuedAt()
    .setExpirationTime(`${SESSION_SECONDS}s`)
    .sign(key(secret));
}
export async function verifySession(token: string | undefined, secret: string) {
  if (!token) return false;
  try {
    const { payload } = await jwtVerify(token, key(secret), {
      algorithms: ["HS256"],
      issuer,
      audience: issuer,
    });
    return payload.sub === "operator" && payload.role === "operator";
  } catch {
    return false;
  }
}
