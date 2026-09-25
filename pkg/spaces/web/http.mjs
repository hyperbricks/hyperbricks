export async function readResponse(response) {
  let result;
  try {
    result = await response.json();
  } catch {
    result = null;
  }
  if (!response.ok || result === null) {
    const fallback = response.status === 429
      ? 'Too many requests. Wait a moment, then try again.'
      : response.ok ? 'Invalid server response. Reload source files and try again.' : `Request failed (${response.status}). Try again or check the server logs.`;
    const error = new Error(typeof result?.error === 'string' ? result.error : fallback);
    error.status = response.status;
    throw error;
  }
  return result;
}
