import { makeApi } from '$lib/api/client';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, params }) => {
	const api = makeApi(fetch);
	try {
		const [details, regions] = await Promise.all([api.service(params.code), api.regions()]);
		return { service: details.service, dimensions: details.dimensions, regions };
	} catch (e) {
		throw error(404, `Service ${params.code} not found`);
	}
};
