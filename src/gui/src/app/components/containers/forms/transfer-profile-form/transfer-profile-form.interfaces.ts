import { FormControl, FormGroup } from '@angular/forms';
import { ProviderType } from '@app/constants/providers.constants';

export type EditorMode = 'add' | 'update';
export type AuthMethodType = 'aws-profile' | 'oidc' | 'static-keys';
export type TestConnectionState = 'idle' | 'testing' | 'ok' | 'error';

export interface TransferProfileForm {
    name: FormControl<string>;
    bucket: FormControl<string>;
    region: FormControl<string>;
    profile: FormControl<string>;

    // S3 provider (Amazon S3 or an S3-compatible service)
    provider: FormControl<ProviderType>;
    pathStyle: FormControl<boolean>;

    // authentication method
    authMethod: FormControl<AuthMethodType>;

    // static access keys (visible when authMethod === 'static-keys')
    accessKeyId: FormControl<string>;
    secretAccessKey: FormControl<string>;
    sessionToken: FormControl<string>;

    // OIDC settings (visible when authMethod === 'oidc')
    oidcIssuerUrl: FormControl<string>;
    oidcClientId: FormControl<string>;
    oidcRoleArn: FormControl<string>;
    oidcScopes: FormControl<string>;
    oidcSessionDurationSeconds: FormControl<number>;
    oidcPersistSession: FormControl<boolean>;
    oidcCustomCaBundle: FormControl<string>;

    // endpoint TLS settings
    tlsCaBundle: FormControl<string>;
    tlsSkipVerify: FormControl<boolean>;

    // advanced settings
    accelerated: FormControl<boolean>;
    storageClass: FormControl<string>;
    checksums: FormGroup<{
        enabled: FormControl<boolean>;
        algorithm: FormControl<string>;
    }>;
    paths: FormGroup<{
        local: FormControl<string>;
        remote: FormControl<string>;
    }>;
    endpoint: FormControl<string>;

    // performance settings
    autoTuning: FormControl<boolean>;
    chunkSize: FormControl<number>;
    threads: FormControl<number>;

    // filter and sort settings
    filter: FormControl<string>;
    maxAge: FormControl<string>;
    fileOrder: FormControl<string[]>;
    enableMetadataFilter: FormControl<boolean>;
}

export interface StorageClass {
    key: string;
    value: string;
}

export interface ChecksumAlgorithm {
    value: string,
    viewValue: string,
}
