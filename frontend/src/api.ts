export async function api(
  path: string,
  org: string,
  body?: Record<string, unknown>,
  key?: string,
) {
  const response = await fetch("/api/v1/" + path, {
    method: body ? "POST" : "GET",
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      "X-Organization-ID": org,
      ...(body ? { "Idempotency-Key": key || crypto.randomUUID() } : {}),
    },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || "Ошибка запроса");
  return data;
}
