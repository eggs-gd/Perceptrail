// The site's facts in one place: its address, this build's channel and how to install it

/** Where the landing is served (GitHub Pages, custom domain) */
export const origin = 'https://perceptrail.eggs.gd';

export const repo = 'https://github.com/eggs-gd/Perceptrail';

export type Channel = 'stable' | 'develop';

/** What a channel installs: the branch its compose file comes from and the images' tag */
export interface Install {
    channel: Channel;
    branch: string;
    tag: string;
    /** Where this channel's landing is, from the root */
    path: string;
}

export const installs: Record<Channel, Install> = {
    stable: {channel: 'stable', branch: 'master', tag: 'latest', path: '/'},
    develop: {channel: 'develop', branch: 'develop', tag: 'develop', path: '/develop/'},
};

/** This build's channel and the monorepo's version it was built from */
export const channel: Channel = __CHANNEL__;
export const version: string = __VERSION__;

/** The commands that install a channel, one per line */
export function commands(install: Install): string[] {
    const lines = [`curl -fsSLO https://raw.githubusercontent.com/eggs-gd/Perceptrail/${install.branch}/compose.yml`];
    if (install.tag !== 'latest') lines.push(`echo PERCEPTRAIL_TAG=${install.tag} > .env`);
    lines.push(
        `docker run --rm --entrypoint cat ghcr.io/eggs-gd/perceptrail-server:${install.tag} /etc/perceptrail/config.yml > config.yml`,
        'docker compose up -d    # then http://<host>:8080',
    );
    return lines;
}
