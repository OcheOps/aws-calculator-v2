import { makeApi } from '$lib/api/client';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const api = makeApi(fetch);
	try {
		const estimate = await api.shared(params.token);
		return { estimate };
	} catch (e) {
		throw error(404, 'Shared estimate not found');
	}
};
