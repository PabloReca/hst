// src/pages/api/loadtest.ts
import type { APIRoute } from 'astro';
import { request } from 'undici';
import { getLoadTestMetricsCollection } from '@/lib/mongodb';

export const prerender = false;

const GO_SERVICE_URL = 'http://localhost:8080';

// Validaciones
function validateLoadTestName(name: string): { valid: boolean; error?: string; normalized?: string } {
  if (!name || typeof name !== 'string') {
    return { valid: false, error: 'Name is required' };
  }
  
  const trimmed = name.trim();
  if (trimmed.length === 0) {
    return { valid: false, error: 'Name cannot be empty' };
  }
  
  if (trimmed.length > 100) {
    return { valid: false, error: 'Name must be less than 100 characters' };
  }
  
  // Normalizar: convertir a snake_case y remover caracteres especiales
  const normalized = trimmed
    .toLowerCase()
    .replace(/\s+/g, '_')
    .replace(/[^a-z0-9_-]/g, '');
  
  if (normalized.length === 0) {
    return { valid: false, error: 'Name must contain alphanumeric characters' };
  }
  
  return { valid: true, normalized };
}

function validateURL(url: string): { valid: boolean; error?: string } {
  if (!url || typeof url !== 'string') {
    return { valid: false, error: 'URL is required' };
  }
  
  try {
    new URL(url);
    return { valid: true };
  } catch {
    return { valid: false, error: 'Invalid URL format' };
  }
}

function validateThreads(threads: number): { valid: boolean; error?: string } {
  if (typeof threads !== 'number' || !Number.isInteger(threads)) {
    return { valid: false, error: 'Threads must be an integer' };
  }
  
  if (threads < 1) {
    return { valid: false, error: 'Threads must be at least 1' };
  }
  
  if (threads > 1000) {
    return { valid: false, error: 'Threads cannot exceed 1000' };
  }
  
  return { valid: true };
}

function validateCallsPerThread(calls: number): { valid: boolean; error?: string } {
  if (typeof calls !== 'number' || !Number.isInteger(calls)) {
    return { valid: false, error: 'CallsPerThread must be an integer' };
  }
  
  if (calls < 1) {
    return { valid: false, error: 'CallsPerThread must be at least 1' };
  }
  
  if (calls > 10000) {
    return { valid: false, error: 'CallsPerThread cannot exceed 10000' };
  }
  
  return { valid: true };
}

// GET - Obtener todos los load tests
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

// POST - Crear y ejecutar load test (proxy al servicio Go)
export const POST: APIRoute = async ({ request }) => {
  try {
    const body = await request.json();
    
    // Validaciones
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
    
    // Verificar si ya existe un load test con ese nombre
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

    // Preparar payload para el servicio Go
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

    // Enviar request al servicio Go usando undici
    console.log(`[API] Sending load test request to Go service: ${normalizedName}`);
    
    const { statusCode, body: responseBody } = await request(
      `${GO_SERVICE_URL}/loadtest`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload)
      }
    );

    const responseData = await responseBody.json();

    if (statusCode === 200 || statusCode === 201) {
      console.log(`[API] Load test started successfully: ${normalizedName}`);
      
      return new Response(
        JSON.stringify({ 
          success: true, 
          ...responseData
        }),
        {
          status: statusCode,
          headers: { 'Content-Type': 'application/json' }
        }
      );
    } else {
      console.error(`[API] Go service returned error: ${statusCode}`, responseData);
      
      return new Response(
        JSON.stringify({ 
          success: false, 
          error: responseData.error || 'Failed to start load test'
        }),
        {
          status: statusCode,
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