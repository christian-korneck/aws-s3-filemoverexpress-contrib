import { describe, it, expect } from 'vitest';
import { S3Provider } from '@gen/es/fme/v1/config_pb';
import {
    presetEndpoint,
    presetFor,
    providerFromProto,
    providerPresets,
    providerToProto,
} from './providers.constants';

describe('presetFor', () => {
    it('falls back to Amazon S3 for a missing provider', () => {
        expect(presetFor(undefined).key).toBe('aws');
        expect(presetFor(null).key).toBe('aws');
        expect(presetFor('').key).toBe('aws');
    });

    it('falls back to the generic S3-compatible preset for an unknown provider', () => {
        expect(presetFor('bogus').key).toBe('other');
    });

    it('returns the matching preset', () => {
        expect(presetFor('hetzner').label).toBe('Hetzner Object Storage');
    });
});

describe('providerFromProto / providerToProto', () => {
    it('maps UNSPECIFIED and missing values to Amazon S3', () => {
        expect(providerFromProto(S3Provider.UNSPECIFIED)).toBe('aws');
        expect(providerFromProto(undefined)).toBe('aws');
    });

    it('maps unknown enum values to the generic preset', () => {
        expect(providerFromProto(999 as S3Provider)).toBe('other');
    });

    it('maps Hetzner', () => {
        expect(providerFromProto(S3Provider.HETZNER)).toBe('hetzner');
        expect(providerToProto('hetzner')).toBe(S3Provider.HETZNER);
    });

    it('maps unknown ids the same way in both directions', () => {
        expect(providerToProto('bogus')).toBe(S3Provider.OTHER);
        expect(providerToProto(undefined)).toBe(S3Provider.AWS);
    });

    it('round-trips every preset', () => {
        for (const preset of providerPresets) {
            expect(providerFromProto(providerToProto(preset.key))).toBe(preset.key);
        }
    });
});

describe('presetEndpoint', () => {
    it('fills in the region for providers whose endpoint carries the signing region', () => {
        expect(presetEndpoint(presetFor('backblaze-b2'), 'us-west-004')).toBe('https://s3.us-west-004.backblazeb2.com');
        expect(presetEndpoint(presetFor('hetzner'), 'nbg1')).toBe('https://nbg1.your-objectstorage.com');
    });

    it('returns nothing while a placeholder is left', () => {
        expect(presetEndpoint(presetFor('cloudflare-r2'), 'auto')).toBe('');
        expect(presetEndpoint(presetFor('backblaze-b2'), '')).toBe('');
        // Spaces signs with us-east-1, which is not the Spaces region in the hostname.
        expect(presetEndpoint(presetFor('digitalocean-spaces'), 'us-east-1')).toBe('');
    });

    it('never fills in example endpoints', () => {
        expect(presetEndpoint(presetFor('minio'), 'us-east-1')).toBe('');
        expect(presetEndpoint(presetFor('other'), 'us-east-1')).toBe('');
        expect(presetEndpoint(presetFor('aws'), 'us-east-1')).toBe('');
    });
});
