export const queryKeys = {
  health: ["health"] as const,
  advertisers: (filters: Record<string, unknown> = {}) =>
    ["advertisers", filters] as const,
  bidders: ["bidders"] as const,
  campaigns: (filters: Record<string, unknown> = {}) =>
    ["campaigns", filters] as const,
  campaign: (id: string) => ["campaign", id] as const,
  snapshot: ["snapshot"] as const,
};
