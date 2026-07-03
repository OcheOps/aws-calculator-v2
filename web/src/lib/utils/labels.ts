// Humanisation of the raw AWS Pricing attribute keys, region codes, and unit
// names. Any casing / word-split logic in the UI should go through here so
// screens stay consistent.

/**
 * Turn a camelCase / PascalCase / snake_case / kebab-case identifier into a
 * sentence-cased phrase.
 *
 *   instanceType         → Instance type
 *   preInstalledSw       → Pre installed sw
 *   OfferingClass        → Offering class
 *   product_family       → Product family
 *   VCPU                 → VCPU (all-caps acronyms preserved)
 */
export function humaniseKey(key: string): string {
	if (!key) return key;
	if (KEY_OVERRIDES[key]) return KEY_OVERRIDES[key];
	// Insert spaces between camel humps + kill separators.
	const spaced = key
		.replace(/[_-]+/g, ' ')
		.replace(/([a-z])([A-Z])/g, '$1 $2')
		.replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
		.replace(/\s+/g, ' ')
		.trim();
	// Sentence case; keep acronyms (all-caps words of ≥2 letters) intact.
	return spaced
		.split(' ')
		.map((word, idx) => {
			if (word.length >= 2 && word === word.toUpperCase()) return word;
			if (idx === 0) return word[0].toUpperCase() + word.slice(1).toLowerCase();
			return word.toLowerCase();
		})
		.join(' ');
}

// A small set of keys where the algorithmic humanisation is ugly.
const KEY_OVERRIDES: Record<string, string> = {
	vcpu: 'vCPU',
	preInstalledSw: 'Pre-installed software',
	operatingSystem: 'Operating system',
	regionCode: 'Region',
	instanceType: 'Instance type',
	instanceFamily: 'Instance family',
	instanceFamilyCategory: 'Instance family category',
	currentGeneration: 'Current generation',
	physicalProcessor: 'Physical processor',
	clockSpeed: 'Clock speed',
	networkPerformance: 'Network performance',
	memory: 'Memory',
	storage: 'Storage',
	tenancy: 'Tenancy',
	capacitystatus: 'Capacity status',
	licenseModel: 'License model',
	productFamily: 'Product family',
	termType: 'Term type',
	usagetype: 'Usage type',
	operation: 'Operation',
	servicecode: 'Service code',
	servicename: 'Service name',
	locationType: 'Location type',
	availabilityzone: 'Availability zone',
	processorArchitecture: 'Processor architecture',
	enhancedNetworkingSupported: 'Enhanced networking',
	ebsOptimized: 'EBS optimised',
	volumeType: 'Volume type',
	volumeApiName: 'Volume API name',
	dedicatedEbsThroughput: 'Dedicated EBS throughput',
	OfferingClass: 'Offering class',
	PurchaseOption: 'Purchase option',
	LeaseContractLength: 'Lease length'
};

/** Friendly display for an AWS region code. */
export function humaniseRegion(code: string): string {
	return REGIONS[code] ?? code;
}

// Non-exhaustive but covers what the demo commonly uses.
const REGIONS: Record<string, string> = {
	'us-east-1': 'US East (N. Virginia) · us-east-1',
	'us-east-2': 'US East (Ohio) · us-east-2',
	'us-west-1': 'US West (N. California) · us-west-1',
	'us-west-2': 'US West (Oregon) · us-west-2',
	'ca-central-1': 'Canada (Central) · ca-central-1',
	'eu-central-1': 'Europe (Frankfurt) · eu-central-1',
	'eu-west-1': 'Europe (Ireland) · eu-west-1',
	'eu-west-2': 'Europe (London) · eu-west-2',
	'eu-west-3': 'Europe (Paris) · eu-west-3',
	'eu-north-1': 'Europe (Stockholm) · eu-north-1',
	'eu-south-1': 'Europe (Milan) · eu-south-1',
	'ap-south-1': 'Asia Pacific (Mumbai) · ap-south-1',
	'ap-northeast-1': 'Asia Pacific (Tokyo) · ap-northeast-1',
	'ap-northeast-2': 'Asia Pacific (Seoul) · ap-northeast-2',
	'ap-southeast-1': 'Asia Pacific (Singapore) · ap-southeast-1',
	'ap-southeast-2': 'Asia Pacific (Sydney) · ap-southeast-2',
	'ap-east-1': 'Asia Pacific (Hong Kong) · ap-east-1',
	'sa-east-1': 'South America (São Paulo) · sa-east-1',
	'me-south-1': 'Middle East (Bahrain) · me-south-1',
	'af-south-1': 'Africa (Cape Town) · af-south-1'
};

/** Friendly display for a pricing unit ("Hrs" → "per hour"). */
export function humaniseUnit(unit: string): string {
	if (!unit) return '';
	const u = unit.toLowerCase();
	if (u === 'hrs' || u === 'hour' || u === 'hours') return 'per hour';
	if (u === 'gb-mo' || u === 'gb-month') return 'per GB-month';
	if (u === 'gb') return 'per GB';
	if (u === 'requests') return 'per request';
	if (u === 'per request-hr') return 'per request-hour';
	return `per ${unit}`;
}
