import {redirect} from '@sveltejs/kit';

// The default view, visible in the URL like any other
export function load() {
    redirect(307, '/v/date');
}
