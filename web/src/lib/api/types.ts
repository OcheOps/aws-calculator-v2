export type Provider = { code: string; name: string };

export type Service = {
	id: number;
	provider_id: number;
	code: string;
	name: string;
	family?: string;
	description?: string;
};

export type Dimension = {
	id: number;
	key: string;
	label: string;
	kind: 'enum' | 'region' | 'numeric' | 'freeform';
	value_type: string;
	is_required: boolean;
	sort_order: number;
};

export type DimensionValue = { value: string; label?: string };
export type Region = { code: string };

export type PriceResponse = {
	sku: string;
	unit: string;
	currency: string;
	price_per_unit: string;
	term_type: string;
	match_quality: 'exact' | 'partial';
	dropped_attributes?: string[];
	fx_rate?: string;
	fx_as_of?: string;
};

export type PricedComponent = {
	service_code: string;
	region_code: string;
	sku: string;
	unit: string;
	quantity: string;
	unit_price: string;
	monthly_cost: string;
	currency: string;
	attributes: Record<string, string>;
};

export type EstimateItem = {
	id: number;
	estimate_id: number;
	service_id: number;
	service_code?: string;
	region_code: string;
	sku: string;
	quantity: string;
	attributes: Record<string, string>;
	unit_price: string;
	currency: string;
	monthly_cost: string;
	line_type: 'legacy' | 'curated';
	service_key?: string;
	user_input?: unknown;
	components?: PricedComponent[];
};

export type Estimate = {
	id: number;
	org_id?: number;
	created_by?: number;
	name: string;
	currency: string;
	notes?: string;
	items?: EstimateItem[];
	total?: string;
	created_at: string;
	updated_at: string;
};

export type User = { id: number; email: string; email_verified_at?: string };

export type PriceDelta = {
	item_id: number;
	old_unit_price: string;
	new_unit_price: string;
	old_monthly: string;
	new_monthly: string;
	currency: string;
};

export type Envelope<T> = {
	data: T;
	meta?: { next_cursor?: string; count?: number };
	error?: { code: string; message: string };
};

export type PricedComponentBody = {
	service_code: string;
	region_code: string;
	sku: string;
	unit: string;
	quantity: string;
	unit_price: string;
	monthly_cost: string;
	currency: string;
	attributes: Record<string, string>;
};

export type AddCuratedItemBody = {
	line_type: 'curated';
	service_id: number;
	service_key: string;
	region_code: string;
	sku: string;
	currency: string;
	user_input: unknown;
	components: PricedComponentBody[];
};
