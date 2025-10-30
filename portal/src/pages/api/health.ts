import type { APIRoute } from 'astro';

export const GET: APIRoute = async ({ params, request }) => {
  return new Response(
    JSON.stringify({
      health: 'ok',
    }),
    {
      status: 200,
      headers: {
        'Content-Type': 'application/json'
      }
    }
  )
};