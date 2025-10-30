// src/pages/api/load-test/[name]/logs.ts
import type { APIRoute } from 'astro';
import { connectToDatabase } from '@/lib/mongodb';

export const prerender = false;

export const GET: APIRoute = async ({ params, url }) => {
    const { name } = params;

    if (!name) {
        return new Response(JSON.stringify({
            success: false,
            error: 'Load test name is required'
        }), {
            status: 400,
            headers: { 'Content-Type': 'application/json' }
        });
    }

    try {
        const db = await connectToDatabase();
        const collectionName = `loadtest_logs_${name}`;
        const collection = db.collection(collectionName);
        
        // Parámetros de query opcionales
        const limitParam = url.searchParams.get('limit');
        const successFilter = url.searchParams.get('success'); // 'true', 'false', o null para todos
        
        const limit = limitParam ? parseInt(limitParam, 10) : 1000;
        
        // Construir filtro
        const filter: any = {};
        if (successFilter !== null) {
            filter.success = successFilter === 'true';
        }
        
        const logs = await collection
            .find(filter)
            .sort({ timestamp: -1 })
            .limit(limit)
            .toArray();

        // Estadísticas adicionales
        const totalCount = await collection.countDocuments({});
        const successCount = await collection.countDocuments({ success: true });
        const failedCount = await collection.countDocuments({ success: false });

        return new Response(JSON.stringify({
            success: true,
            data: logs,
            stats: {
                total: totalCount,
                successful: successCount,
                failed: failedCount,
                successRate: totalCount > 0 ? (successCount / totalCount) * 100 : 0
            }
        }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' }
        });
    } catch (error) {
        console.error('[API] Failed to fetch load test logs:', error);
        
        return new Response(JSON.stringify({
            success: false,
            error: String(error)
        }), {
            status: 500,
            headers: { 'Content-Type': 'application/json' }
        });
    }
};