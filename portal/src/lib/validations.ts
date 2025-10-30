export interface ValidationResult {
  valid: boolean;
  error?: string;
  normalized?: string;
}

export function validateHealthCheckName(name: string): ValidationResult {
  if (!name || name.trim().length === 0) {
    return { valid: false, error: 'Name is required' };
  }

  const normalized = name.trim().toLowerCase().replace(/\s+/g, '-');
  
  if (!/^[a-z-]+$/.test(normalized)) {
    return { 
      valid: false, 
      error: 'Name can only contain letters (a-z) and hyphens (-)' 
    };
  }

  if (normalized.startsWith('-') || normalized.endsWith('-') || normalized.includes('--')) {
    return { 
      valid: false, 
      error: 'Invalid hyphen placement' 
    };
  }

  return { valid: true, normalized };
}

export function validateURL(url: string): ValidationResult {
  if (!url || url.trim().length === 0) {
    return { valid: false, error: 'URL is required' };
  }

  try {
    new URL(url);
    return { valid: true };
  } catch {
    return { valid: false, error: 'Invalid URL format' };
  }
}

export function validateInterval(interval: number): ValidationResult {
  if (!interval || interval < 1) {
    return { valid: false, error: 'Interval must be at least 1 second' };
  }

  if (interval > 86400) {
    return { valid: false, error: 'Interval cannot exceed 24 hours (86400 seconds)' };
  }

  return { valid: true };
}

export function validateStatusCode(statusCode: number): ValidationResult {
  if (!statusCode || statusCode < 100 || statusCode > 599) {
    return { valid: false, error: 'Status code must be between 100 and 599' };
  }

  return { valid: true };
}


export function validateLoadTestName(name: string): ValidationResult {
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
  
  const normalized = trimmed
    .toLowerCase()
    .replace(/\s+/g, '_')
    .replace(/[^a-z0-9_-]/g, '');
  
  if (normalized.length === 0) {
    return { valid: false, error: 'Name must contain alphanumeric characters' };
  }
  
  return { valid: true, normalized };
}

export function validateThreads(threads: number): ValidationResult {
  if (typeof threads !== 'number' || threads % 1 !== 0) {
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

export function validateCallsPerThread(calls: number): ValidationResult {
  if (typeof calls !== 'number' || calls % 1 !== 0) {
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