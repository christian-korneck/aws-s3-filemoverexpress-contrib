import { describe, it, expect } from 'vitest';
import { TransferProfile } from '@classes/config';
import { validateRegion, validTransferProfileExists } from './config-utils';

const awsRegions = ['us-east-1', 'eu-west-1'];

function profile(overrides: Partial<TransferProfile>): TransferProfile {
    return TransferProfile.fromJson({
        name: 'p',
        bucket: 'media',
        region: 'us-east-1',
        profile: '',
        accelerated: false,
        fileOrder: [],
        enableMetadataFilter: true,
        storageClass: 'standard',
        paths: {local: '', remote: ''},
        chunkSize: 25,
        threads: 10,
        maxAge: '',
        filter: '',
        checksums: {enabled: false, algorithm: 'none'},
        autoTuning: true,
        endpoint: '',
        ...overrides,
    });
}

describe('validateRegion', () => {
    it('checks Amazon S3 regions against the AWS Region list', () => {
        expect(validateRegion('eu-west-1', awsRegions)).toBe('');
        expect(validateRegion('auto', awsRegions)).toBe('Invalid AWS region');
        expect(validateRegion('auto', awsRegions, 'aws')).toBe('Invalid AWS region');
        expect(validateRegion('', awsRegions, 'aws')).toBe('AWS region is required');
    });

    it('accepts any region name for S3-compatible providers', () => {
        expect(validateRegion('auto', awsRegions, 'cloudflare-r2')).toBe('');
        expect(validateRegion('fsn1', awsRegions, 'hetzner')).toBe('');
        expect(validateRegion('us-west-004', awsRegions, 'backblaze-b2')).toBe('');
        expect(validateRegion('', awsRegions, 'minio')).toBe('Region is required');
    });
});

describe('validTransferProfileExists', () => {
    it('counts a non-AWS profile with a provider-specific region as valid', () => {
        expect(validTransferProfileExists({r2: profile({provider: 'cloudflare-r2', region: 'auto'})}, awsRegions)).toBe(true);
    });

    it('rejects an Amazon S3 profile with a non-AWS region', () => {
        expect(validTransferProfileExists({aws: profile({region: 'auto'})}, awsRegions)).toBe(false);
    });
});
