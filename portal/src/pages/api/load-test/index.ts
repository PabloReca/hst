import type { APIRoute } from 'astro';
import { getLoadTestMetricsCollection } from '@/lib/mongodb';
import { 
  validateLoadTestName,
  validateURL,
  validateThreads,
  validateCallsPerThread
} from '@/lib/validations';

export const prerender = false;

const GO_SERVICE_URL = 'http://localhost:8080';

export const GET: APIRoute = async () => {
  try {
    const collection = await getLoadTestMetricsCollection();
    
    const loadTests = await collection
      .find({})
      .sort({ timestamp: -1 })
      .toArray();
    
    return new Response(
      JSON.stringify({ 
        success: true, 
        data: loadTests 
      }),
      {
        status: 200,
        headers: { 'Content-Type': 'application/json' }
      }
    );
  } catch (error) {
    console.error('[API] Failed to fetch load tests:', error);
    
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
    
    const nameValidation = validateLoadTestName(body.name);
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

    const threadsValidation = validateThreads(body.threads);
    if (!threadsValidation.valid) {
      return new Response(
        JSON.stringify({ success: false, error: threadsValidation.error }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const callsValidation = validateCallsPerThread(body.callsPerThread);
    if (!callsValidation.valid) {
      return new Response(
        JSON.stringify({ success: false, error: callsValidation.error }),
        { status: 400, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const normalizedName = nameValidation.normalized!;
    
    const collection = await getLoadTestMetricsCollection();
    const existing = await collection.findOne({ name: normalizedName });
    
    if (existing) {
      return new Response(
        JSON.stringify({ 
          success: false, 
          error: `A load test with name '${normalizedName}' already exists. Please use a different name.` 
        }),
        { status: 409, headers: { 'Content-Type': 'application/json' } }
      );
    }

    const payload = {
      name: normalizedName,
      url: body.url,
      method: body.method || 'GET',
      headers: body.headers || {},
      body: body.body || '',
      callsPerThread: body.callsPerThread,
      threads: body.threads,
      timeout: body.timeout || 30,
      expectedStatusCode: body.expectedStatusCode || 200
    };

    console.log(`[API] Sending load test request to Go service: ${normalizedName}`);
    
    const goResponse = await fetch(
      `${GO_SERVICE_URL}/loadtest`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload)
      }
    );

    const responseData = await goResponse.json();

    if (goResponse.status === 200 || goResponse.status === 201) {
      console.log(`[API] Load test started successfully: ${normalizedName}`);
      
      return new Response(
        JSON.stringify({ 
          success: true, 
          ...responseData
        }),
        {
          status: goResponse.status,
          headers: { 'Content-Type': 'application/json' }
        }
      );
    } else {
      console.error(`[API] Go service returned error: ${goResponse.status}`, responseData);
      
      return new Response(
        JSON.stringify({ 
          success: false, 
          error: responseData.error || 'Failed to start load test'
        }),
        {
          status: goResponse.status,
          headers: { 'Content-Type': 'application/json' }
        }
      );
    }

  } catch (error) {
    console.error('[API] Failed to create load test:', error);
    
    return new Response(
      JSON.stringify({ 
        success: false, 
        error: error instanceof Error ? error.message : String(error)
      }),
      {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      }
    );
  }
};