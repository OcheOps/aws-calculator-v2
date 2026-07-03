import { makeApi } from '$lib/api/client';
import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch, cookies, params, url }) => {
	const editToken = url.searchParams.get('edit') ?? undefined;
	const sessionCookie = cookies.get('session');
	const api = makeApi(fetch, sessionCookie ? `session=${sessionCookie}` : undefined);
	try {
		const id = Number(params.id);
		const estimate = editToken
			? await api.getEstimate(id, editToken)
			: await api.getEstimate(id);
		return { estimate, editToken: editToken ?? null };
	} catch (e) {
		throw error(404, (e as Error).message);
	}
};
