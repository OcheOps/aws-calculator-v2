export function debounce<F extends (...args: any[]) => void>(fn: F, ms: number): F {
	let t: ReturnType<typeof setTimeout> | undefined;
	return ((...args: any[]) => {
		if (t) clearTimeout(t);
		t = setTimeout(() => fn(...args), ms);
	}) as F;
}
