import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

const API_URL = process.env.PUBLIC_API_URL ?? 'http://api:8080';

export const load: PageServerLoad = async ({ url, fetch, cookies }) => {
	const token = url.searchParams.get('token');
	if (!token) throw redirect(302, '/auth/request');
	try {
		const res = await fetch(API_URL + '/api/v1/auth/verify', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token })
		});
		const setCookie = res.headers.get('set-cookie');
		if (setCookie) {
			const match = /session=([^;]+)/.exec(setCookie);
			if (match) {
				cookies.set('session', match[1], {
					path: '/',
					httpOnly: true,
					sameSite: 'lax',
					secure: process.env.API_ENV === 'production',
					maxAge: 30 * 24 * 3600
				});
			}
		}
		if (!res.ok) return { ok: false, error: `Verification failed (${res.status})` };
	} catch (e) {
		return { ok: false, error: (e as Error).message };
	}
	throw redirect(302, '/');
};
