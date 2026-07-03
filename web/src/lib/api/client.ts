import { ApiError } from './errors';
import type {
	Envelope,
	Provider,
	Service,
	Dimension,
	DimensionValue,
	Region,
	PriceResponse,
	Estimate,
	EstimateItem,
	User,
	PriceDelta
} from './types';

const isBrowser = typeof window !== 'undefined';

function apiBase(): string {
	if (isBrowser) return '';
	return process.env.PUBLIC_API_URL ?? 'http://api:8080';
}

type FetchImpl = typeof fetch;

export type Api = ReturnType<typeof makeApi>;

export function makeApi(fetchImpl: FetchImpl = fetch, cookieHeader?: string) {
	async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
		const headers: Record<string, string> = {
			'Content-Type': 'application/json',
			...((init.headers as Record<string, string>) ?? {})
		};
		if (cookieHeader) headers['Cookie'] = cookieHeader;
		const res = await fetchImpl(apiBase() + '/api/v1' + path, {
			credentials: 'include',
			...init,
			headers
		});
		const body = (await res.json().catch(() => ({}))) as Envelope<T>;
		if (!res.ok) {
			const err = body?.error;
			throw new ApiError(res.status, err?.code ?? 'error', err?.message ?? res.statusText);
		}
		return body.data;
	}
	return {
		providers: () => request<Provider[]>('/providers'),
		services: (q: { provider?: string; q?: string; cursor?: string; limit?: number }) => {
			const s = new URLSearchParams();
			Object.entries(q).forEach(([k, v]) => v !== undefined && s.set(k, String(v)));
			return request<Service[]>('/services?' + s.toString());
		},
		service: (code: string, provider = 'aws') =>
			request<{ service: Service; dimensions: Dimension[] }>(
				`/services/${code}?provider=${provider}`
			),
		dimensionValues: (code: string, dim: string, provider = 'aws') =>
			request<DimensionValue[]>(
				`/services/${code}/dimension-values?provider=${provider}&dim=${encodeURIComponent(dim)}`
			),
		regions: (provider = 'aws') => request<Region[]>(`/regions?provider=${provider}`),
		currencies: () => request<string[]>('/currencies'),
		price: (in_: {
			service: string;
			region: string;
			term?: string;
			currency?: string;
			attributes: Record<string, string>;
			provider?: string;
		}) => {
			const s = new URLSearchParams({
				service: in_.service,
				region: in_.region,
				provider: in_.provider ?? 'aws'
			});
			if (in_.term) s.set('term', in_.term);
			if (in_.currency) s.set('currency', in_.currency);
			for (const [k, v] of Object.entries(in_.attributes)) s.set(`attr[${k}]`, v);
			return request<PriceResponse>(`/price?${s.toString()}`);
		},
		requestLink: (email: string) =>
			request<{ accepted: boolean }>('/auth/request-link', {
				method: 'POST',
				body: JSON.stringify({ email })
			}),
		verifyLink: (token: string, claim_tokens: string[] = []) =>
			request<{ user: User; expires_at: string }>('/auth/verify', {
				method: 'POST',
				body: JSON.stringify({ token, claim_tokens })
			}),
		logout: () => request<{ ok: boolean }>('/auth/logout', { method: 'POST' }),
		me: () => request<User>('/auth/me'),
		createEstimate: (body: { name: string; currency?: string; notes?: string }) =>
			request<{ estimate: Estimate; edit_token?: string }>('/estimates', {
				method: 'POST',
				body: JSON.stringify(body)
			}),
		listEstimates: () => request<Estimate[]>('/estimates'),
		getEstimate: (id: number, editToken?: string) =>
			request<Estimate>(`/estimates/${id}`, {
				headers: editToken ? { 'X-Edit-Token': editToken } : {}
			}),
		addItem: (
			id: number,
			body: {
				service_id: number;
				region_code: string;
				sku: string;
				quantity: string;
				attributes: Record<string, string>;
				unit_price: string;
				unit: string;
				currency: string;
			},
			editToken?: string
		) =>
			request<EstimateItem>(`/estimates/${id}/items`, {
				method: 'POST',
				body: JSON.stringify(body),
				headers: editToken ? { 'X-Edit-Token': editToken } : {}
			}),
		deleteItem: (id: number, itemId: number, editToken?: string) =>
			request<{ deleted: boolean }>(`/estimates/${id}/items/${itemId}`, {
				method: 'DELETE',
				headers: editToken ? { 'X-Edit-Token': editToken } : {}
			}),
		reprice: (id: number, apply = false, editToken?: string) =>
			request<PriceDelta[]>(`/estimates/${id}/reprice${apply ? '?apply=true' : ''}`, {
				method: 'POST',
				headers: editToken ? { 'X-Edit-Token': editToken } : {}
			}),
		share: (id: number, editToken?: string) =>
			request<{ share_url: string; share_token: string }>(`/estimates/${id}/share`, {
				method: 'POST',
				headers: editToken ? { 'X-Edit-Token': editToken } : {}
			}),
		shared: (token: string) => request<Estimate>(`/shared/${token}`)
	};
}

export const api = makeApi();
