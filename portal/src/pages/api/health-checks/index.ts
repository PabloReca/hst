import type { APIRoute } from 'astro';
import { getHealthChecksCollection } from '@/lib/mongodb';
import { 
  validateHealthCheckName, 
  validateURL, 
  validateInterval,
  validateStatusCode 
} from '@/lib/validations';

export const prerender = false;

export const GET: APIRoute = async () => {
  try {
    const collection = await getHealthChecksCollection();
    const healthChecks = await collection.find({ status: 'active' }).toArray();
    
    return new Response(
      JSON.stringify({ 
        success: true, 
        data: healthChecks 
      }),
      {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      }
    );
  } catch (error) {
    console.error('[API] Failed to fetch health checks:', error);
    
    return new Response(
      JSON.stringify({ 
        success: false, 
        error: String(error) 
      }),
      {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      }
    );
  }
};

export const POST: APIRoute = async ({ request }) => {
  try {
    const body = await request.json();
    
    const nameValidation = validateHealthCheckName(body.name);
    if (!nameValidation.valid) {
      return new Response(
        JSON.stringify({ success: false, error: nameValidation.error }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const urlValidation = validateURL(body.url);
    if (!urlValidation.valid) {
      return new Response(
        JSON.stringify({ success: false, error: urlValidation.error }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const intervalValidation = validateInterval(body.interval);
    if (!intervalValidation.valid) {
      return new Response(
        JSON.stringify({ success: false, error: intervalValidation.error }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const statusCodeValidation = validateStatusCode(body.statusCode);
    if (!statusCodeValidation.valid) {
      return new Response(
        JSON.stringify({ success: false, error: statusCodeValidation.error }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const normalizedName = nameValidation.normalized!;
    
    const collection = await getHealthChecksCollection();
    
    const existing = await collection.findOne({ name: normalizedName });
    if (existing) {
      return new Response(
        JSON.stringify({ 
          success: false, 
          error: 'A health check with this name already exists' 
        }),
        { status: 409, headers: { 'Content-Type': 'application/json' } }
      );
    }
    
    const result = await collection.insertOne({
      ...body,
      name: normalizedName,
      createdAt: new Date(),
      status: 'active'
    });
    
    console.log(`[API] Health check created successfully - Name: ${normalizedName}`);
    
    return new Response(
      JSON.stringify({ 
        success: true, 
        id: result.insertedId,
        name: normalizedName,
        message: 'Health check created'
      }),
      {
        status: 201,
        headers: { 'Content-Type': 'application/json' }
      }
    );
  } catch (error) {
    console.error('[API] Failed to create health check:', error);
    
    return new Response(
      JSON.stringify({ 
        success: false, 
        error: String(error) 
      }),
      {
        status: 400,
        headers: { 'Content-Type': 'application/json' }
      }
    );
  }
};