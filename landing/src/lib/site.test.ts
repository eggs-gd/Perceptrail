import {describe, expect, it} from 'vitest';
import {commands, installs} from './site';

describe('commands', () => {
    it('stable: the release compose file and images, no tag to set', () => {
        const lines = commands(installs.stable);
        expect(lines[0]).toContain('/master/compose.yml');
        expect(lines.some((l) => l.includes('PERCEPTRAIL_TAG'))).toBe(false);
        expect(lines[1]).toContain('perceptrail-server:latest');
    });

    it('develop: its compose file, the develop tag in .env and in the config template', () => {
        const lines = commands(installs.develop);
        expect(lines[0]).toContain('/develop/compose.yml');
        expect(lines[1]).toBe('echo PERCEPTRAIL_TAG=develop > .env');
        expect(lines[2]).toContain('perceptrail-server:develop');
    });
});
