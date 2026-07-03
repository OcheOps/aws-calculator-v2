import type { RequestHandler } from './$types';
import { error } from '@sveltejs/kit';

const API_URL = process.env.PUBLIC_API_URL ?? 'http://api:8080';

export const GET: RequestHandler = async ({ params, url, cookies, fetch }) => {
	const format = url.searchParams.get('format') ?? 'pdf';
	const edit = url.searchParams.get('edit') ?? undefined;
	const sessionCookie = cookies.get('session');
	const headers: Record<string, string> = {};
	if (edit) headers['X-Edit-Token'] = edit;
	if (sessionCookie) headers['Cookie'] = `session=${sessionCookie}`;

	const upstream = await fetch(
		`${API_URL}/api/v1/estimates/${params.id}/export?format=${format}`,
		{ headers }
	);
	if (!upstream.ok) throw error(upstream.status, `Export failed (${upstream.status})`);
	const contentType = upstream.headers.get('Content-Type') ?? 'application/octet-stream';
	const disposition =
		upstream.headers.get('Content-Disposition') ??
		`attachment; filename="estimate.${format}"`;
	const body = await upstream.arrayBuffer();
	return new Response(body, {
		headers: { 'Content-Type': contentType, 'Content-Disposition': disposition }
	});
};
