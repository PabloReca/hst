// src/lib/validations.ts

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