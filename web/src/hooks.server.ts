import type { Handle } from '@sveltejs/kit';

const API_URL = process.env.PUBLIC_API_URL ?? 'http://api:8080';

export const handle: Handle = async ({ event, resolve }) => {
	const sessionCookie = event.cookies.get('session');
	event.locals.user = null;
	if (sessionCookie) {
		try {
			const res = await event.fetch(API_URL + '/api/v1/auth/me', {
				headers: { cookie: `session=${sessionCookie}` }
			});
			if (res.ok) {
				const body = await res.json();
				event.locals.user = body.data;
			}
		} catch {
			// Best-effort; treat as unauthenticated.
		}
	}
	return resolve(event);
};
