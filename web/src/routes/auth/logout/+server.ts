import { redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const API_URL = process.env.PUBLIC_API_URL ?? 'http://api:8080';

export const POST: RequestHandler = async ({ cookies, fetch }) => {
	const sessionCookie = cookies.get('session');
	if (sessionCookie) {
		try {
			await fetch(API_URL + '/api/v1/auth/logout', {
				method: 'POST',
				headers: { cookie: `session=${sessionCookie}` }
			});
		} catch {
			// Best-effort logout on the backend; clearing the cookie locally is the important part.
		}
	}
	cookies.delete('session', { path: '/' });
	throw redirect(302, '/');
};
