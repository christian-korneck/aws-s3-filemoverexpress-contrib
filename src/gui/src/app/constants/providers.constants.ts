import { S3Provider } from '@gen/es/fme/v1/config_pb';

/**
 * Identifier of the S3 implementation a Remote Configuration talks to. Mirrors the Go
 * `configtypes.S3Provider` string values so the same ids appear in configuration.yaml.
 */
export type ProviderType = 'aws' | 'minio' | 'cloudflare-r2' | 'backblaze-b2' | 'wasabi' | 'digitalocean-spaces' | 'hetzner' | 'other';

export interface ProviderPreset {
    key: ProviderType;
    label: string;
    proto: S3Provider;
    isAws: boolean;
    /**
     * Endpoint URL pattern for the provider. {region} is replaced with the region when
     * substituteRegion is set; a template that still contains a placeholder afterwards is only shown
     * as the input placeholder, never filled in as a value.
     */
    endpointTemplate: string;
    /** Example endpoint shown as the input placeholder for self-hosted stores without a fixed pattern. */
    endpointExample: string;
    /** True when the {region} in the endpoint template is the same value as the signing region. */
    substituteRegion: boolean;
    endpointHint: string;
    defaultRegion: string;
    regionHint: string;
    pathStyle: boolean;
    supportsOidc: boolean;
    supportsStorageClass: boolean;
}

export const DEFAULT_PROVIDER: ProviderType = 'aws';

/**
 * Presets mirror the daemon's provider table (src/cli/types/configtypes/providers.go). Everything a
 * preset fills in stays editable in the form.
 */
export const providerPresets: ProviderPreset[] = [
    {
        key: 'aws',
        label: 'Amazon S3',
        proto: S3Provider.AWS,
        isAws: true,
        endpointTemplate: '',
        endpointExample: '',
        substituteRegion: false,
        endpointHint: 'Optional. Only needed for an S3 Access Point or VPC endpoint.',
        defaultRegion: '',
        regionHint: 'The AWS Region that your S3 bucket is located in.',
        pathStyle: false,
        supportsOidc: true,
        supportsStorageClass: true,
    },
    {
        key: 'minio',
        label: 'MinIO',
        proto: S3Provider.MINIO,
        isAws: false,
        endpointTemplate: '',
        endpointExample: 'https://minio.example.com:9000',
        substituteRegion: false,
        endpointHint: 'The MinIO server API address (usually port 9000, not the console port 9001).',
        defaultRegion: 'us-east-1',
        regionHint: 'MinIO accepts any region name; us-east-1 is the default.',
        pathStyle: true,
        supportsOidc: false,
        supportsStorageClass: true,
    },
    {
        key: 'cloudflare-r2',
        label: 'Cloudflare R2',
        proto: S3Provider.CLOUDFLARE_R2,
        isAws: false,
        endpointTemplate: 'https://{account_id}.r2.cloudflarestorage.com',
        endpointExample: '',
        substituteRegion: false,
        endpointHint: 'Replace {account_id} with your Cloudflare account ID.',
        defaultRegion: 'auto',
        regionHint: 'R2 uses the region "auto".',
        pathStyle: false,
        supportsOidc: false,
        supportsStorageClass: true,
    },
    {
        key: 'backblaze-b2',
        label: 'Backblaze B2',
        proto: S3Provider.BACKBLAZE_B2,
        isAws: false,
        endpointTemplate: 'https://s3.{region}.backblazeb2.com',
        endpointExample: '',
        substituteRegion: true,
        endpointHint: 'Shown as "Endpoint" on your bucket details page, e.g. s3.us-west-004.backblazeb2.com.',
        defaultRegion: 'us-west-004',
        regionHint: 'Must match the region in the endpoint, e.g. us-west-004.',
        pathStyle: false,
        supportsOidc: false,
        supportsStorageClass: false,
    },
    {
        key: 'wasabi',
        label: 'Wasabi',
        proto: S3Provider.WASABI,
        isAws: false,
        endpointTemplate: 'https://s3.{region}.wasabisys.com',
        endpointExample: '',
        substituteRegion: true,
        endpointHint: 'Use the service URL for your bucket region, e.g. s3.eu-central-1.wasabisys.com.',
        defaultRegion: 'us-east-1',
        regionHint: 'Must match the region in the endpoint.',
        pathStyle: false,
        supportsOidc: false,
        supportsStorageClass: false,
    },
    {
        key: 'digitalocean-spaces',
        label: 'DigitalOcean Spaces',
        proto: S3Provider.DIGITALOCEAN_SPACES,
        isAws: false,
        endpointTemplate: 'https://{region}.digitaloceanspaces.com',
        endpointExample: '',
        substituteRegion: false,
        endpointHint: 'Replace {region} with the Spaces region, e.g. nyc3 or fra1.',
        defaultRegion: 'us-east-1',
        regionHint: 'DigitalOcean recommends us-east-1 for S3 clients regardless of the Spaces region.',
        pathStyle: false,
        supportsOidc: false,
        supportsStorageClass: false,
    },
    {
        key: 'hetzner',
        label: 'Hetzner Object Storage',
        proto: S3Provider.HETZNER,
        isAws: false,
        endpointTemplate: 'https://{region}.your-objectstorage.com',
        endpointExample: '',
        substituteRegion: true,
        endpointHint: 'Replace {region} with the bucket location: fsn1 (Falkenstein), nbg1 (Nuremberg) or hel1 (Helsinki).',
        defaultRegion: 'fsn1',
        regionHint: 'Use the location code of your bucket (fsn1, nbg1 or hel1); it must match the endpoint.',
        pathStyle: false,
        supportsOidc: false,
        supportsStorageClass: false,
    },
    {
        key: 'other',
        label: 'Other S3-compatible',
        proto: S3Provider.OTHER,
        isAws: false,
        endpointTemplate: '',
        endpointExample: 'https://s3.example.com:9000',
        substituteRegion: false,
        endpointHint: 'Full URL of the S3 API, e.g. https://s3.example.com:9000.',
        defaultRegion: 'us-east-1',
        regionHint: 'Most S3-compatible stores accept us-east-1.',
        pathStyle: true,
        supportsOidc: false,
        supportsStorageClass: true,
    },
];

/**
 * Value the daemon returns instead of a stored secret access key or session token. Sending it back
 * unchanged tells the daemon to keep the stored value (only while the access key ID is unchanged).
 */
export const REDACTED_SECRET = '***REDACTED***';

/**
 * Returns the preset for a provider id. A missing id means a configuration written before providers
 * existed (Amazon S3); an unknown id falls back to the generic S3-compatible preset, matching
 * providerFromProto for unknown enum values.
 */
export function presetFor(provider: string | undefined | null): ProviderPreset {
    if (!provider) {
        return providerPresets[0];
    }
    return providerPresets.find((preset) => preset.key === provider) ?? otherPreset();
}

export function providerToProto(provider: string | undefined | null): S3Provider {
    return presetFor(provider).proto;
}

/** UNSPECIFIED (legacy configurations) maps to Amazon S3; unknown enum values map to the generic preset. */
export function providerFromProto(provider: S3Provider | undefined | null): ProviderType {
    if (provider === undefined || provider === null || provider === S3Provider.UNSPECIFIED) {
        return DEFAULT_PROVIDER;
    }
    return providerPresets.find((preset) => preset.proto === provider)?.key ?? 'other';
}

/**
 * The endpoint to fill in when the preset is chosen, or '' when the template needs values only the
 * user knows (e.g. {account_id}). Those templates are shown as the input placeholder instead.
 */
export function presetEndpoint(preset: ProviderPreset, region: string): string {
    let endpoint = preset.endpointTemplate;
    if (preset.substituteRegion && region) {
        endpoint = endpoint.replaceAll('{region}', region);
    }
    return /[{}]/.test(endpoint) ? '' : endpoint;
}

function otherPreset(): ProviderPreset {
    return providerPresets.find((preset) => preset.key === 'other') as ProviderPreset;
}
