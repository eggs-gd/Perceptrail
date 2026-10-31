import {defineEnvVars} from '@sveltejs/kit/env';

// The environment the app reads ($app/env/public): the API's path, fixed at build
export const variables = defineEnvVars({PUBLIC_API_PATH: {public: true, static: true}});
