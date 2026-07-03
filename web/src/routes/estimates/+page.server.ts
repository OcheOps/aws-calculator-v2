import { makeApi } from '$lib/api/client';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, cookies, locals }) => {
	if (!locals.user) throw redirect(302, '/auth/request');
	const sessionCookie = cookies.get('session');
	const api = makeApi(fetch, sessionCookie ? `session=${sessionCookie}` : undefined);
	try {
		const estimates = await api.listEstimates();
		return { estimates: estimates ?? [], error: null };
	} catch (e) {
		return { estimates: [], error: (e as Error).message };
	}
};
