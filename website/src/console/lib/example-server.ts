export function parseExampleServerUrls(raw: string | undefined): string[] {
  if (!raw) {
    return [];
  }
  const unique = new Set<string>();
  for (const part of raw.split(",")) {
    const trimmed = part.trim();
    if (!trimmed) {
      continue;
    }
    const normalized = normalizeBaseUrl(trimmed);
    if (!normalized) {
      continue;
    }
    unique.add(normalized);
  }
  return Array.from(unique);
}

export function normalizeBaseUrl(url: string): string {
  return url.replace(/\/+$/, "");
}
