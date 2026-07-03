import type { User } from '$lib/api/types';

let user = $state<User | null>(null);

export const session = {
	get user() {
		return user;
	},
	set(u: User | null) {
		user = u;
	}
};
