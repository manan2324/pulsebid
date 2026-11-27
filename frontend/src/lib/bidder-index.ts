export function getBidderIndex(
  endpoint: string,
  { basePort, count }: { basePort: number; count: number },
) {
  try {
    const url = new URL(endpoint);
    const port = Number(url.port || (url.protocol === "https:" ? 443 : 80));
    const index = port - basePort;
    return /^https?:\/\//.test(endpoint) &&
      endpoint === endpoint.trim() &&
      /^https?:$/.test(url.protocol) &&
      Number.isInteger(index) &&
      index >= 0 &&
      index < count
      ? index
      : null;
  } catch {
    return null;
  }
}
