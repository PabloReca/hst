import type { APIRoute } from 'astro';
import { connectToDatabase } from '@/lib/mongodb';
export const prerender = false;

export const GET: APIRoute = async ({ params }) => {
    const { name } = params;

    if (!name) {
        return new Response(JSON.stringify({
            success: false,
            error: 'Health check name is required'
        }), {
            status: 400,
            headers: { 'Content-Type': 'application/json' }
        });
    }

    try {
        const db = await connectToDatabase();
        const collectionName = `healthcheck_${name}`;
        const collection = db.collection(collectionName);
        
        const logs = await collection
            .find({})
            .sort({ timestamp: -1 })
            .limit(100)
            .toArray();

        return new Response(JSON.stringify({
            success: true,
            data: logs
        }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' }
        });
    } catch (error) {
        console.error('[API] Failed to fetch logs:', error);
        
        return new Response(JSON.stringify({
            success: false,
            error: String(error)
        }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' }
        });
    }
};