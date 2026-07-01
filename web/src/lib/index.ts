export const API_URL = (globalThis as { process?: { env?: Record<string, string> } }).process?.env?.PUBLIC_API_URL ?? 'http://localhost:8080';
