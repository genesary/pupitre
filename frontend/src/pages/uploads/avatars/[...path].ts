import type { APIRoute } from 'astro';

const USER_API = process.env.USER_API ?? 'http://localhost:8081';

export const GET: APIRoute = async ({ params }) => {
  const path = params.path ?? '';
  const upstream = `${USER_API}/uploads/avatars/${path}`;

  let res: Response;
  try {
    res = await fetch(upstream);
  } catch (err) {
    const msg = err instanceof Error ? err.message : 'Upstream unavailable';
    return new Response(msg, { status: 502 });
  }

  const headers = new Headers();
  for (const [k, v] of res.headers.entries()) {
    const lower = k.toLowerCase();
    if (lower === 'transfer-encoding' || lower === 'connection') continue;
    headers.set(k, v);
  }

  return new Response(res.body, { status: res.status, headers });
};
