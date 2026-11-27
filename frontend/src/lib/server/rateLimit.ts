const attempts = new Map<string, { count: number; expires: number }>();
export function allowLogin(ip: string, now = Date.now()) {
  for (const [key, value] of attempts)
    if (value.expires <= now) attempts.delete(key);
  const entry = attempts.get(ip) ?? { count: 0, expires: now + 60_000 };
  if (entry.count >= 5) return false;
  // Bound memory; fail closed if a deployment is receiving too many distinct clients.
  if (!attempts.has(ip) && attempts.size >= 10_000) return false;
  entry.count++;
  attempts.set(ip, entry);
  return true;
}
