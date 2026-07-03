import { makeApi } from '$lib/api/client';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, url }) => {
	const api = makeApi(fetch);
	const q = url.searchParams.get('q') ?? '';
	try {
		const services = await api.services({ q, limit: 200 });
		return { services, q, error: null };
	} catch (e) {
		return { services: [], q, error: (e as Error).message };
	}
};
